package corerulesetgen

import (
	"path/filepath"
	"strconv"

	"github.com/networking-incubator/coraza-kubernetes-operator/internal/rulesets"
)

// NamedYAML is one generated ConfigMap manifest (full document YAML).
type NamedYAML struct {
	Name string
	Doc  string
}

// BuildStats counts per-file rule processing outcomes.
type BuildStats struct {
	Processed int
	Skipped   int
}

// ConfFileResult holds one .conf outcome for logging (mirrors buildConfigMapYAML).
type ConfFileResult struct {
	BaseName   string
	Warns      []string
	ConfigName string
	YAML       string
	SkipReason string
}

// ManifestBundle is the full multi-doc output before writing to stdout.
type ManifestBundle struct {
	BaseConfigMapYAML string
	ExtraConfigMaps   []NamedYAML
	SecretDoc         string
	RuleSetDoc        string
	Stats             BuildStats
	ConfFileResults   []ConfFileResult
}

// Build produces base ConfigMap, per-.conf ConfigMaps, optional Secret, and RuleSet from a
// parsed [CRSVersion]. It does not read stderr or write to stdout.
func Build(opts Options, scan ScanResult, ver CRSVersion) (*ManifestBundle, error) {
	opts = mergeWASMUnsupportedIDs(opts)

	baseYAML, baseRulesScalar := baseRulesYAML(ver.Normalized, ver.Setup, opts.IncludeTestRule)
	baseYAML = injectNamespaceInBaseConfigMapYAML(baseYAML, opts.Namespace)
	if err := checkPayloadSize(baseRulesScalar, "base-rules", opts); err != nil {
		return nil, err
	}

	confResults := make([]ConfFileResult, 0, len(scan.ConfPaths))
	var extra []NamedYAML
	var names []string
	processed, skipped := 0, 0

	for _, p := range scan.ConfPaths {
		name, cmYAML, skipReason, warns, berr := buildConfigMapYAML(p, opts)
		confResults = append(confResults, ConfFileResult{
			BaseName:   filepath.Base(p),
			Warns:      warns,
			ConfigName: name,
			YAML:       cmYAML,
			SkipReason: skipReason,
		})
		if berr != nil {
			return nil, berr
		}
		if cmYAML != "" {
			extra = append(extra, NamedYAML{Name: name, Doc: cmYAML})
			names = append(names, name)
			processed++
		} else {
			skipped++
		}
	}

	secretDoc := ""
	if len(scan.DataPaths) > 0 {
		var serr error
		secretDoc, serr = buildDataSecretYAML(scan.DataPaths, opts)
		if serr != nil {
			return nil, serr
		}
	}

	rs := rulesetYAML(names, opts, len(scan.DataPaths) > 0)

	return &ManifestBundle{
		BaseConfigMapYAML: baseYAML,
		ExtraConfigMaps:   extra,
		SecretDoc:         secretDoc,
		RuleSetDoc:        rs,
		Stats:             BuildStats{Processed: processed, Skipped: skipped},
		ConfFileResults:   confResults,
	}, nil
}

// mergeWASMUnsupportedIDs returns a copy of opts with the operator's WASM
// unsupported rule IDs merged into IgnoreRuleIDs, unless the caller opted
// out via IncludeWASMUnsupportedRules.
func mergeWASMUnsupportedIDs(opts Options) Options {
	if opts.IncludeWASMUnsupportedRules {
		return opts
	}
	merged := make(map[string]struct{}, len(opts.IgnoreRuleIDs)+len(rulesets.AllUnsupportedRuleIDs()))
	for id := range opts.IgnoreRuleIDs {
		merged[id] = struct{}{}
	}
	for _, id := range rulesets.AllUnsupportedRuleIDs() {
		merged[strconv.Itoa(id)] = struct{}{}
	}
	opts.IgnoreRuleIDs = merged
	return opts
}
