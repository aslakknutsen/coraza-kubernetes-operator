package corerulesetgen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	secDirectiveLine = regexp.MustCompile(`^(SecRule|SecAction|SecMarker)\b`)
	ruleIDRe         = regexp.MustCompile(`id:(\d+)`)
)

func splitIntoRules(content string) []string {
	lines := strings.Split(content, "\n")
	blocks := make([]string, 0, len(lines))
	var current []string
	inMultiline := false

	for _, line := range lines {
		stripped := strings.TrimRight(line, "\r\n")
		if inMultiline {
			current = append(current, line)
			if !strings.HasSuffix(stripped, "\\") {
				inMultiline = false
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
			}
			continue
		}
		if !strings.HasPrefix(stripped, "#") && secDirectiveLine.MatchString(stripped) {
			current = []string{line}
			if strings.HasSuffix(stripped, "\\") {
				inMultiline = true
			} else {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
			}
			continue
		}
		blocks = append(blocks, line)
	}
	if len(current) > 0 {
		blocks = append(blocks, strings.Join(current, "\n"))
	}
	return blocks
}

func extractRuleID(ruleText string) string {
	m := ruleIDRe.FindStringSubmatch(ruleText)
	if m != nil {
		return m[1]
	}
	return "unknown"
}

func processFileContent(path string, ignoreIDs map[string]struct{}, ignorePM bool) (string, []string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("read %s: %w", path, err)
	}
	content := strings.TrimRight(strings.ToValidUTF8(string(raw), ""), "\n\r")
	if !strings.Contains(content, "SecRule") && !strings.Contains(content, "SecAction") {
		return "", nil, nil
	}

	var warns []string
	blocks := splitIntoRules(content)
	var filtered []string
	base := filepath.Base(path)

	for _, block := range blocks {
		stripped := strings.TrimSpace(block)
		if stripped != "" && !strings.HasPrefix(stripped, "#") && strings.HasPrefix(stripped, "Sec") {
			if ignorePM && strings.HasPrefix(stripped, "SecRule") && strings.Contains(block, "@pmFromFile") {
				rid := extractRuleID(block)
				warns = append(warns, fmt.Sprintf("  [warn] Ignored rules in %s:\n    - Rule ID: %s (@pmFromFile not supported)\n", base, rid))
				continue
			}
			rid := extractRuleID(block)
			if _, drop := ignoreIDs[rid]; drop {
				warns = append(warns, fmt.Sprintf("  [warn] Ignored rules in %s:\n    - Rule ID: %s (Rule ID in ignore list)\n", base, rid))
				continue
			}
			filtered = append(filtered, block)
		} else {
			filtered = append(filtered, block)
		}
	}
	return strings.Join(filtered, "\n"), warns, nil
}

func buildConfigMapYAML(path string, opts Options) (name, yamlOut, skipReason string, warns []string, err error) {
	base := filepath.Base(path)
	rawName, err := generateConfigMapName(base)
	if err != nil {
		return "", "", err.Error(), nil, err
	}
	name = opts.NamePrefix + rawName + opts.NameSuffix
	if name == "" {
		return "", "", "invalid empty ConfigMap name", nil, fmt.Errorf("empty ConfigMap name after prefix/suffix")
	}

	processed, w, err := processFileContent(path, opts.IgnoreRuleIDs, opts.IgnorePMFromFile)
	if err != nil {
		return "", "", "", nil, err
	}
	warns = w
	if strings.TrimSpace(processed) == "" {
		return "", "", "No SecRule or SecAction directives found", warns, nil
	}

	indented := indentRulesMultiline(processed)
	payload := indented + "\n"
	if err := checkPayloadSize(payload, name, opts); err != nil {
		return "", "", "", warns, err
	}
	yamlOut = formatConfigMapYAML(name, opts.Namespace, indented)
	return name, yamlOut, "", warns, nil
}
