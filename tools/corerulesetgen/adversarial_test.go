package corerulesetgen

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdversarial_ParseCRSVersion_empty(t *testing.T) {
	_, err := ParseCRSVersion("")
	require.Error(t, err)
}

func TestAdversarial_ParseCRSVersion_vOnly(t *testing.T) {
	_, err := ParseCRSVersion("v")
	require.Error(t, err)
}

func TestAdversarial_ParseCRSVersion_tripleVPrefix(t *testing.T) {
	_, err := ParseCRSVersion("vvv4.24.1")
	require.Error(t, err)
}

func TestAdversarial_ParseCRSVersion_doubleVPrefix(t *testing.T) {
	_, err := ParseCRSVersion("vv4.24.1")
	require.Error(t, err)
}

func TestAdversarial_ParseCRSVersion_negativeComponent(t *testing.T) {
	_, err := ParseCRSVersion("-1.0.0")
	require.Error(t, err)
}

func TestAdversarial_ParseCRSVersion_nonNumeric(t *testing.T) {
	_, err := ParseCRSVersion("abc")
	require.Error(t, err)
}

func TestAdversarial_ParseCRSVersion_veryLong(t *testing.T) {
	long := strings.Repeat("1.", 500) + "0"
	v, err := ParseCRSVersion(long)
	require.NoError(t, err)
	require.NotEmpty(t, v.Normalized)
}

func TestAdversarial_ParseCRSVersion_whitespacePadded(t *testing.T) {
	v, err := ParseCRSVersion("  v4.24.1  ")
	require.NoError(t, err)
	require.Equal(t, "4.24.1", v.Normalized)
}

func TestAdversarial_Scan_emptyDirNoConf(t *testing.T) {
	tmp := t.TempDir()
	s, err := Scan(tmp)
	require.NoError(t, err)
	require.Empty(t, s.ConfPaths)
	require.Empty(t, s.DataPaths)
	require.False(t, s.PMFromFileRefs)
}

func TestAdversarial_Generate_rulesPathIsFileNotDir(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "x.conf")
	require.NoError(t, os.WriteFile(f, []byte("SecRule ARGS \"@rx a\" \"id:1,pass\"\n"), 0o644))
	_, err := Generate(io.Discard, Options{
		RulesDir: f,
		Version:  "4.0.0",
		Stderr:   io.Discard,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a directory")
}

func TestAdversarial_Generate_nilStderrUsesDiscard(t *testing.T) {
	tmp := t.TempDir()
	_, err := Generate(io.Discard, Options{
		RulesDir: tmp,
		Version:  "4.0.0",
	})
	require.NoError(t, err)
}

func TestAdversarial_WriteManifests_nilBundlePanics(t *testing.T) {
	require.Panics(t, func() {
		_ = WriteManifests(&bytes.Buffer{}, nil)
	})
}

func TestAdversarial_WriteManifests_emptyBundleStillWrites(t *testing.T) {
	var b ManifestBundle
	var out bytes.Buffer
	err := WriteManifests(&out, &b)
	require.NoError(t, err)
	require.NotEmpty(t, out.String())
}

func TestAdversarial_checkPayloadSize_rejectsHugePayload(t *testing.T) {
	huge := strings.Repeat("x", maxRulesPayloadBytes+1)
	err := checkPayloadSize(huge, "test-cm", Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "limit")
}

func TestAdversarial_checkPayloadSize_skipSizeCheckAllowsHuge(t *testing.T) {
	huge := strings.Repeat("x", maxRulesPayloadBytes+1)
	err := checkPayloadSize(huge, "test-cm", Options{SkipSizeCheck: true})
	require.NoError(t, err)
}

func TestAdversarial_duplicateConfFilesSameContent(t *testing.T) {
	tmp := t.TempDir()
	content := `SecRule ARGS "@rx a" "id:1,pass"` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "a.conf"), []byte(content), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "b.conf"), []byte(content), 0o644))
	ver := mustParseCRSVersion(t, "4.0.0")
	scan, err := Scan(tmp)
	require.NoError(t, err)
	require.Len(t, scan.ConfPaths, 2)
	bundle, err := Build(Options{
		RulesDir:       tmp,
		Version:        "4.0.0",
		RuleSetName:    "rs",
		DataSecretName: "ds",
	}, scan, ver)
	require.NoError(t, err)
	require.Len(t, bundle.ExtraConfigMaps, 2)
}

func TestAdversarial_processFileContent_unicodeAndSpecialChars(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "u.conf")
	content := "SecRule ARGS \"@rx \u2022\" \"id:42,pass\"\n"
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	out, _, err := processFileContent(p, nil, false)
	require.NoError(t, err)
	require.Contains(t, out, "id:42")
}

func TestAdversarial_processFileContent_veryLongLine(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "long.conf")
	longArg := strings.Repeat("A", 10000)
	content := `SecRule ARGS "` + longArg + `" "id:7,pass"` + "\n"
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	out, _, err := processFileContent(p, nil, false)
	require.NoError(t, err)
	require.Contains(t, out, "id:7")
}

func TestAdversarial_ignoreRuleIDs_nonexistentIDsNoError(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "z.conf"), []byte(`SecRule ARGS "@rx a" "id:1,pass"`+"\n"), 0o644))
	ver := mustParseCRSVersion(t, "4.0.0")
	scan, err := Scan(tmp)
	require.NoError(t, err)
	bundle, err := Build(Options{
		RulesDir:       tmp,
		Version:        "4.0.0",
		RuleSetName:    "rs",
		DataSecretName: "ds",
		IgnoreRuleIDs:  map[string]struct{}{"99999": {}},
	}, scan, ver)
	require.NoError(t, err)
	require.Contains(t, bundle.ExtraConfigMaps[0].Doc, "id:1")
}

func TestAdversarial_pmFromFile_warningWhenNoDataFilesAndNotIgnoring(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "p.conf"), []byte(`SecRule ARGS "@pmFromFile x.data" "id:1,pass"`+"\n"), 0o644))
	var errBuf bytes.Buffer
	_, err := Generate(io.Discard, Options{
		RulesDir: tmp,
		Version:  "4.0.0",
		Stderr:   &errBuf,
	})
	require.NoError(t, err)
	require.Contains(t, errBuf.String(), "@pmFromFile")
}

func TestAdversarial_pmFromFile_noWarningWhenIgnoring(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "p.conf"), []byte(`SecRule ARGS "@pmFromFile x.data" "id:1,pass"`+"\n"), 0o644))
	var errBuf bytes.Buffer
	_, err := Generate(io.Discard, Options{
		RulesDir:         tmp,
		Version:          "4.0.0",
		IgnorePMFromFile: true,
		Stderr:           &errBuf,
	})
	require.NoError(t, err)
	require.NotContains(t, errBuf.String(), "warning: @pmFromFile references found")
}

func TestAdversarial_injectNamespaceInBaseConfigMapYAML_missingMarkerNoChange(t *testing.T) {
	doc := "kind: ConfigMap\nmetadata:\n  name: other\n"
	out := injectNamespaceInBaseConfigMapYAML(doc, "ns")
	require.Equal(t, doc, out)
}

func TestAdversarial_rulesetYAML_emptyConfigMapNames(t *testing.T) {
	y := rulesetYAML(nil, Options{RuleSetName: "r", DataSecretName: "d"}, false)
	require.Contains(t, y, "name: r")
	require.Contains(t, y, "- name: base-rules")
}

func TestAdversarial_formatRuleSetYAML_emptyName(t *testing.T) {
	y := formatRuleSetYAML("", "", nil, "")
	require.Contains(t, y, "kind: RuleSet")
}

func TestAdversarial_Generate_nonExistentRulesDir(t *testing.T) {
	_, err := Generate(io.Discard, Options{
		RulesDir: "/nonexistent/coreruleset/rules/path",
		Version:  "4.0.0",
		Stderr:   io.Discard,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rules directory")
}

func TestAdversarial_Generate_malformedVersion(t *testing.T) {
	tmp := t.TempDir()
	_, err := Generate(io.Discard, Options{
		RulesDir: tmp,
		Version:  "not semver",
		Stderr:   io.Discard,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "CoreRuleSet version")
}

func TestAdversarial_Scan_symlinkConfFileReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test is Unix-oriented")
	}
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real.conf")
	require.NoError(t, os.WriteFile(real, []byte("SecRule ARGS \"@rx a\" \"id:1,pass\"\n"), 0o644))
	link := filepath.Join(tmp, "via-link.conf")
	require.NoError(t, os.Symlink(real, link))
	s, err := Scan(tmp)
	require.NoError(t, err)
	require.Len(t, s.ConfPaths, 2)
}

func TestAdversarial_Scan_permissionDeniedOnConf(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may bypass file mode bits for read")
	}
	tmp := t.TempDir()
	p := filepath.Join(tmp, "locked.conf")
	require.NoError(t, os.WriteFile(p, []byte("SecRule ARGS \"@rx a\" \"id:1,pass\"\n"), 0o644))
	require.NoError(t, os.Chmod(p, 0))
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	_, err := Scan(tmp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read")
}

func TestAdversarial_checkPayloadSize_exactlyAtLimitAccepted(t *testing.T) {
	s := strings.Repeat("x", maxRulesPayloadBytes)
	require.NoError(t, checkPayloadSize(s, "cm", Options{}))
}

func TestAdversarial_checkPayloadSize_oneByteOverLimitRejected(t *testing.T) {
	s := strings.Repeat("x", maxRulesPayloadBytes+1)
	err := checkPayloadSize(s, "cm", Options{})
	require.Error(t, err)
}

func TestAdversarial_checkSecretStringData_totalBoundary(t *testing.T) {
	// Per-value max and total max both use ~900KiB constants; two half-size values should pass total check.
	half := maxSecretStringDataTotalBytes / 2
	entries := map[string]string{"a": strings.Repeat("y", half), "b": strings.Repeat("z", half)}
	require.NoError(t, checkSecretStringDataSize("sec", entries, Options{}))
	entries["c"] = "extra"
	require.Error(t, checkSecretStringDataSize("sec", entries, Options{}))
}

func TestAdversarial_processFileContent_emptyFile(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "empty.conf")
	require.NoError(t, os.WriteFile(p, []byte(""), 0o644))
	out, warns, err := processFileContent(p, nil, false)
	require.NoError(t, err)
	require.Empty(t, warns)
	require.Empty(t, out)
}

func TestAdversarial_processFileContent_binaryWithoutSecMarkers(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "noise.conf")
	require.NoError(t, os.WriteFile(p, []byte{0, 1, 2, 255, 0}, 0o644))
	out, _, err := processFileContent(p, nil, false)
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestAdversarial_formatConfigMapYAML_multilineDocumentMarkersInContent(t *testing.T) {
	// Literal block should preserve lines that look like YAML document separators.
	indented := indentRulesMultiline("SecRule ARGS \"@rx x\" \"id:1,pass\"\n---\nSecRule ARGS \"@rx y\" \"id:2,pass\"")
	y := formatConfigMapYAML("rules-a", "", indented)
	require.Contains(t, y, "---")
	require.Contains(t, y, "id:1")
}

func TestAdversarial_buildConfigMapYAML_rejectsPayloadOneByteOverLimit(t *testing.T) {
	tmp := t.TempDir()
	// Craft a single-line SecRule whose indented payload exceeds maxRulesPayloadBytes.
	padding := strings.Repeat("Z", maxRulesPayloadBytes)
	content := "SecRule ARGS \"" + padding + "\" \"id:1,pass\"\n"
	p := filepath.Join(tmp, "huge.conf")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	_, _, _, _, err := buildConfigMapYAML(p, Options{RuleSetName: "rs", DataSecretName: "ds"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "limit")
}

func TestAdversarial_ParseCRSVersion_equivalentForms(t *testing.T) {
	a, err := ParseCRSVersion("4.24.1")
	require.NoError(t, err)
	b, err := ParseCRSVersion("v4.24.1")
	require.NoError(t, err)
	require.Equal(t, a.Normalized, b.Normalized)
	require.Equal(t, a.Setup, b.Setup)
}
