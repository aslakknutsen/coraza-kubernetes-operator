package corerulesetgen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func stderrf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

func stderrln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}

// Options configures CoreRuleSet manifest generation.
type Options struct {
	RulesDir         string
	Version          string
	IgnoreRuleIDs    map[string]struct{}
	IgnorePMFromFile bool
	IncludeTestRule  bool
	RuleSetName      string
	Namespace        string
	DataSecretName   string
	NamePrefix       string
	NameSuffix       string
	DryRun           bool
	SkipSizeCheck    bool
	Stderr           io.Writer
}

// Result holds a short summary after a successful Generate.
type Result struct {
	ConfigMapCount int
	HasSecret      bool
	RuleSetName    string
	Namespace      string
}

func applyDefaults(opts Options) Options {
	if opts.RuleSetName == "" {
		opts.RuleSetName = "default-ruleset"
	}
	if opts.DataSecretName == "" {
		opts.DataSecretName = "coreruleset-data"
	}
	return opts
}

// Generate walks RulesDir and writes multi-document YAML to out (stdout).
func Generate(out io.Writer, opts Options) (*Result, error) {
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	opts = applyDefaults(opts)

	if opts.DryRun {
		stderrln(opts.Stderr, "dry-run: no objects sent to cluster")
	}

	ver, err := ParseCRSVersion(opts.Version)
	if err != nil {
		return nil, err
	}

	rulesPath := filepath.Clean(opts.RulesDir)
	st, err := os.Stat(rulesPath)
	if err != nil {
		return nil, fmt.Errorf("rules directory: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", rulesPath)
	}

	scan, err := Scan(rulesPath)
	if err != nil {
		return nil, err
	}

	bundle, err := Build(opts, scan, ver)
	if err != nil {
		return nil, err
	}

	stderrf(opts.Stderr, "Found %d .conf files in %s\n", len(scan.ConfPaths), rulesPath)
	if len(scan.DataPaths) > 0 {
		stderrf(opts.Stderr, "Found %d .data files in %s\n", len(scan.DataPaths), rulesPath)
	}

	stderrf(opts.Stderr, "\nProcessing %d rule files...\n\n", len(scan.ConfPaths))

	for _, r := range bundle.ConfFileResults {
		stderrf(opts.Stderr, "Processing: %s\n", r.BaseName)
		for _, w := range r.Warns {
			stderrf(opts.Stderr, "%s", w)
		}
		if r.YAML != "" {
			stderrf(opts.Stderr, "  [ok] Generated ConfigMap: %s\n", r.ConfigName)
		} else {
			stderrf(opts.Stderr, "  [skip] Skipped: %s\n", r.SkipReason)
		}
	}

	if len(scan.DataPaths) > 0 {
		stderrf(opts.Stderr, "\nProcessing %d data files...\n\n", len(scan.DataPaths))
		for _, p := range scan.DataPaths {
			stderrf(opts.Stderr, "Processing: %s\n", filepath.Base(p))
			stderrf(opts.Stderr, "  [ok] Added to Secret: %s\n", opts.DataSecretName)
		}
	}

	writeGenerateSummary(opts.Stderr, scan.PMFromFileRefs, len(scan.DataPaths) == 0, bundle.Stats.Processed, bundle.Stats.Skipped, len(bundle.ExtraConfigMaps), len(scan.DataPaths), opts.DataSecretName)

	if err := WriteManifests(out, bundle); err != nil {
		return nil, err
	}

	stderrf(opts.Stderr, "generated RuleSet %q", opts.RuleSetName)
	if opts.Namespace != "" {
		stderrf(opts.Stderr, " in namespace %q", opts.Namespace)
	}
	stderrf(opts.Stderr, ": %d ConfigMap(s), secret=%v\n", len(bundle.ExtraConfigMaps)+1, len(scan.DataPaths) > 0)

	return &Result{
		ConfigMapCount: len(bundle.ExtraConfigMaps) + 1,
		HasSecret:      len(scan.DataPaths) > 0,
		RuleSetName:    opts.RuleSetName,
		Namespace:      opts.Namespace,
	}, nil
}

func writeGenerateSummary(stderr io.Writer, pmFromFileRefs, noDataFiles bool, processed, skipped, configMapCount, dataFileCount int, dataSecretName string) {
	if pmFromFileRefs && noDataFiles {
		stderrln(stderr, "warning: @pmFromFile references found under the rules directory but no .data files were emitted into a Secret; add matching .data files or use --ignore-pmFromFile if the operator should not load pmFromFile data.")
	}
	stderrf(stderr, "\n%s\n", strings.Repeat("=", 60))
	stderrln(stderr, "Summary:")
	stderrln(stderr, "  Base rules: 1 (bundled)")
	stderrf(stderr, "  Processed: %d rule files\n", processed)
	stderrf(stderr, "  Skipped: %d rule files\n", skipped)
	stderrf(stderr, "  Total ConfigMaps: %d\n", configMapCount+1)
	stderrf(stderr, "  Data files: %d\n", dataFileCount)
	if dataFileCount > 0 {
		stderrf(stderr, "  Data Secret: %s\n", dataSecretName)
	}
	stderrf(stderr, "%s\n\n", strings.Repeat("=", 60))
}
