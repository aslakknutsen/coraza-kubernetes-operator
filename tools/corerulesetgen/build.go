package corerulesetgen

import (
	"path/filepath"
	"strconv"
	"strings"

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
	opts = mergeUnsupportedIDs(opts)

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

// UnsupportedRuleProfileWASM is the profile name for the WASM engine.
const UnsupportedRuleProfileWASM = "wasm"

// mergeUnsupportedIDs returns a copy of opts with unsupported rule IDs
// for the selected profile merged into IgnoreRuleIDs. When no profile
// matches (empty string or "none"), no IDs are merged.
func mergeUnsupportedIDs(opts Options) Options {
	ids := unsupportedIDsForProfile(opts.IgnoreUnsupportedRules)
	if len(ids) == 0 {
		opts.autoIgnoredIDs = nil
		return opts
	}

	userIgnore := opts.IgnoreRuleIDs
	merged := make(map[string]struct{}, len(opts.IgnoreRuleIDs)+len(ids))
	for id := range opts.IgnoreRuleIDs {
		merged[id] = struct{}{}
	}
	autoOnly := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		sid := strconv.Itoa(id)
		merged[sid] = struct{}{}
		if userIgnore == nil {
			autoOnly[sid] = struct{}{}
			continue
		}
		if _, fromUser := userIgnore[sid]; !fromUser {
			autoOnly[sid] = struct{}{}
		}
	}
	opts.IgnoreRuleIDs = merged
	opts.autoIgnoredIDs = autoOnly
	return opts
}

// unsupportedIDsForProfile returns the unsupported rule IDs for a given
// profile name. Returns nil for unknown or empty profiles.
func unsupportedIDsForProfile(profile string) []int {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case UnsupportedRuleProfileWASM:
		return rulesets.AllUnsupportedRuleIDs()
	default:
		return nil
	}
}
