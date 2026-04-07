package corerulesetgen

import (
	"path/filepath"
)

// NamedYAML is one generated RuleSource manifest (full document YAML).
type NamedYAML struct {
	Name string
	Doc  string
}

// BuildStats counts per-file rule processing outcomes.
type BuildStats struct {
	Processed int
	Skipped   int
}

// ConfFileResult holds one .conf outcome for logging.
type ConfFileResult struct {
	BaseName   string
	Warns      []string
	SourceName string
	YAML       string
	SkipReason string
}

// ManifestBundle is the full multi-doc output before writing to stdout.
type ManifestBundle struct {
	BaseRuleSourceYAML string
	ExtraRuleSources   []NamedYAML
	DataRuleSourceDoc  string
	RuleSetDoc         string
	Stats              BuildStats
	ConfFileResults    []ConfFileResult
}

// Build produces base RuleSource, per-.conf RuleSources, optional Data RuleSource,
// and RuleSet from a parsed [CRSVersion]. It does not read stderr or write to stdout.
func Build(opts Options, scan ScanResult, ver CRSVersion) (*ManifestBundle, error) {
	baseYAML, baseRulesScalar := baseRulesYAML(ver.Normalized, ver.Setup, opts.IncludeTestRule)
	baseYAML = injectNamespaceInBaseRuleSourceYAML(baseYAML, opts.Namespace)
	if err := checkPayloadSize(baseRulesScalar, "base-rules", opts); err != nil {
		return nil, err
	}

	confResults := make([]ConfFileResult, 0, len(scan.ConfPaths))
	var extra []NamedYAML
	var names []string
	processed, skipped := 0, 0

	for _, p := range scan.ConfPaths {
		name, rsYAML, skipReason, warns, berr := buildRuleSourceYAML(p, opts)
		confResults = append(confResults, ConfFileResult{
			BaseName:   filepath.Base(p),
			Warns:      warns,
			SourceName: name,
			YAML:       rsYAML,
			SkipReason: skipReason,
		})
		if berr != nil {
			return nil, berr
		}
		if rsYAML != "" {
			extra = append(extra, NamedYAML{Name: name, Doc: rsYAML})
			names = append(names, name)
			processed++
		} else {
			skipped++
		}
	}

	dataDoc := ""
	if len(scan.DataPaths) > 0 {
		var serr error
		dataDoc, serr = buildDataRuleSourceYAML(scan.DataPaths, opts)
		if serr != nil {
			return nil, serr
		}
	}

	rs := rulesetYAML(names, opts, len(scan.DataPaths) > 0)

	return &ManifestBundle{
		BaseRuleSourceYAML: baseYAML,
		ExtraRuleSources:   extra,
		DataRuleSourceDoc:  dataDoc,
		RuleSetDoc:         rs,
		Stats:              BuildStats{Processed: processed, Skipped: skipped},
		ConfFileResults:    confResults,
	}, nil
}
