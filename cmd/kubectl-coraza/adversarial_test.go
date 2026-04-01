/*
Copyright Coraza Kubernetes Operator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdversarial_root_unknownSubcommand fails when the user invokes a non-existent subcommand.
func TestAdversarial_root_unknownSubcommand(t *testing.T) {
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"nonexistent-subcommand"})
	err := cmd.Execute()
	require.Error(t, err)
}

// TestAdversarial_generate_missingCorerulesetSubcommand documents that "generate" alone succeeds
// (no RunE on the parent); users must pass "coreruleset" explicitly to run generation.
func TestAdversarial_generate_missingCorerulesetSubcommand(t *testing.T) {
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate"})
	err := cmd.Execute()
	require.NoError(t, err)
}

// TestAdversarial_coreruleset_missingRulesDir required flag.
func TestAdversarial_coreruleset_missingRulesDir(t *testing.T) {
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--version", "4.24.1"})
	err := cmd.Execute()
	require.Error(t, err)
}

// TestAdversarial_coreruleset_missingVersion required flag.
func TestAdversarial_coreruleset_missingVersion(t *testing.T) {
	dir := testdataDir(t, "minimal")
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", dir})
	err := cmd.Execute()
	require.Error(t, err)
}

// TestAdversarial_coreruleset_emptyRulesDirFlag documents filepath.Clean("") resolving to "."
// (current working directory), which may succeed without a clear error.
func TestAdversarial_coreruleset_emptyRulesDirFlag(t *testing.T) {
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", "", "--version", "4.24.1"})
	err := cmd.Execute()
	require.NoError(t, err)
}

// TestAdversarial_coreruleset_emptyVersionFlag empty required version.
func TestAdversarial_coreruleset_emptyVersionFlag(t *testing.T) {
	dir := testdataDir(t, "minimal")
	cmd, _, stderr := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", dir, "--version", ""})
	err := cmd.Execute()
	require.Error(t, err)
	_ = stderr
}

// TestAdversarial_coreruleset_positionalArgsIgnoredOrRejected extra positional tokens after flags.
func TestAdversarial_coreruleset_positionalArgsIgnoredOrRejected(t *testing.T) {
	dir := testdataDir(t, "minimal")
	cmd, stdout, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", dir, "--version", "4.24.1", "extra", "tokens"})
	err := cmd.Execute()
	// Cobra may accept unknown args depending on SilenceErrors; document behavior.
	if err == nil {
		assert.Contains(t, stdout.String(), "kind: RuleSet")
	} else {
		require.Error(t, err)
	}
}

// TestAdversarial_coreruleset_dryRunNotClient does not enable dry-run stderr banner.
func TestAdversarial_coreruleset_dryRunNotClient(t *testing.T) {
	dir := testdataDir(t, "minimal")
	cmd, _, stderr := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", dir, "--version", "4.24.1", "--dry-run", "server"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.NotContains(t, stderr.String(), "dry-run: no objects sent to cluster")
}

// TestAdversarial_coreruleset_ignoreRulesInvalidIDs demonstrates a bug: genCRS writes
// "Ignoring rule IDs" to os.Stderr instead of cmd.ErrOrStderr(), so callers
// capturing command stderr (e.g. tests, wrappers) never see the message.
func TestAdversarial_coreruleset_ignoreRulesInvalidIDs(t *testing.T) {
	dir := testdataDir(t, "minimal")
	cmd, stdout, stderr := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", dir, "--version", "4.24.1", "--ignore-rules", "999999,not-a-number"})
	err := cmd.Execute()
	require.NoError(t, err)
	// BUG: This assertion fails because genCRS writes to os.Stderr (line 128 of main.go)
	// instead of cmd.ErrOrStderr(). The message is lost when stderr is redirected.
	assert.Contains(t, stderr.String(), "Ignoring rule IDs",
		"BUG: genCRS writes 'Ignoring rule IDs' to os.Stderr instead of cmd.ErrOrStderr()")
	assert.Contains(t, stdout.String(), "kind: RuleSet")
}

// TestAdversarial_coreruleset_doubleHyphenStyle still parses.
func TestAdversarial_coreruleset_doubleHyphenStyle(t *testing.T) {
	dir := testdataDir(t, "minimal")
	cmd, stdout, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir=" + dir, "--version=4.24.1"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "kind: RuleSet")
}

// TestAdversarial_root_helpNoPanic requests help on root (may error if usage is written to Err).
func TestAdversarial_root_helpNoPanic(t *testing.T) {
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"-h"})
	err := cmd.Execute()
	// Help often returns nil or typed ErrHelp; accept either without panic.
	_ = err
}

// TestAdversarial_generate_coreruleset_helpNoPanic.
func TestAdversarial_generate_coreruleset_helpNoPanic(t *testing.T) {
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "-h"})
	err := cmd.Execute()
	_ = err
}

// TestAdversarial_coreruleset_rulesDirIsFile not a directory.
func TestAdversarial_coreruleset_rulesDirIsFile(t *testing.T) {
	dir := testdataDir(t, "minimal")
	confPath := filepath.Join(dir, "simple.conf")
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", confPath, "--version", "4.24.1"})
	err := cmd.Execute()
	require.Error(t, err)
}

// TestAdversarial_coreruleset_versionStripsLeadingV ensures v-prefixed versions match bare semver in output.
func TestAdversarial_coreruleset_versionStripsLeadingV(t *testing.T) {
	dir := testdataDir(t, "minimal")
	cmd, stdout, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", dir, "--version", "v4.24.1"})
	err := cmd.Execute()
	require.NoError(t, err)
	out := stdout.String()
	assert.Contains(t, out, "OWASP_CRS/4.24.1")
	assert.NotContains(t, out, "OWASP_CRS/v4.24.1")
}

// TestAdversarial_coreruleset_versionBareSemver matches stripping behavior for non-prefixed input.
func TestAdversarial_coreruleset_versionBareSemver(t *testing.T) {
	dir := testdataDir(t, "minimal")
	cmd, stdout, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", dir, "--version", "4.24.1"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "OWASP_CRS/4.24.1")
}

// TestAdversarial_coreruleset_invalidVersionTripleV rejects more than one leading "v".
func TestAdversarial_coreruleset_invalidVersionTripleV(t *testing.T) {
	dir := testdataDir(t, "minimal")
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", dir, "--version", "vvv4.24.1"})
	err := cmd.Execute()
	require.Error(t, err)
}

// TestAdversarial_coreruleset_invalidVersionOnlyV rejects a lone "v".
func TestAdversarial_coreruleset_invalidVersionOnlyV(t *testing.T) {
	dir := testdataDir(t, "minimal")
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", dir, "--version", "v"})
	err := cmd.Execute()
	require.Error(t, err)
}

// TestAdversarial_coreruleset_nonExistentRulesDir surfaces filesystem error from Generate.
func TestAdversarial_coreruleset_nonExistentRulesDir(t *testing.T) {
	cmd, _, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", "/no/such/rules/dir/under/tmp/qa", "--version", "4.24.1"})
	err := cmd.Execute()
	require.Error(t, err)
}

// TestAdversarial_coreruleset_dirWithNoConfFiles succeeds with base ConfigMap only (empty scan).
func TestAdversarial_coreruleset_dirWithNoConfFiles(t *testing.T) {
	tmp := t.TempDir()
	cmd, stdout, _ := newTestCommand(t)
	cmd.SetArgs([]string{"generate", "coreruleset", "--rules-dir", tmp, "--version", "4.0.0"})
	err := cmd.Execute()
	require.NoError(t, err)
	out := stdout.String()
	assert.Equal(t, 1, strings.Count(out, "kind: ConfigMap"), "only base-rules ConfigMap when no *.conf files")
	assert.Contains(t, out, "name: base-rules")
	assert.Contains(t, out, "kind: RuleSet")
}
