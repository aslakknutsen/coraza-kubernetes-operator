package corerulesetgen

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// Approximate max size for a single ConfigMap "rules" entry or one Secret stringData value
// to stay under etcd limits (~1 MiB per object / value).
const maxRulesPayloadBytes = 900 * 1024

// Approximate max total size of all Secret stringData values combined.
const maxSecretStringDataTotalBytes = 900 * 1024

var (
	validVersionRe = regexp.MustCompile(`^\d+(\.\d+)*$`)
	nameSanitizeRe = regexp.MustCompile(`[^a-z0-9.-]+`)
)

// CRSVersion is the normalized CoreRuleSet version string and the digits-only form
// used in SecAction (tx.crs_setup_version).
type CRSVersion struct {
	Normalized string
	Setup      string
}

// ParseCRSVersion parses a CoreRuleSet version (e.g. "4.24.1" or "v4.24.1").
func ParseCRSVersion(v string) (CRSVersion, error) {
	norm, setup, err := normalizeCRSVersion(v)
	if err != nil {
		return CRSVersion{}, err
	}
	return CRSVersion{Normalized: norm, Setup: setup}, nil
}

func normalizeCRSVersion(v string) (normalized, setupDigits string, err error) {
	normalized = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if !validVersionRe.MatchString(normalized) {
		return "", "", fmt.Errorf("invalid CoreRuleSet version format %q: expected digits and dots (e.g. 4.24.1 or v4.24.1)", v)
	}
	setupDigits = strings.ReplaceAll(normalized, ".", "")
	return normalized, setupDigits, nil
}

func checkPayloadSize(rulesBlock string, objectName string, opts Options) error {
	if opts.SkipSizeCheck {
		return nil
	}
	if len(rulesBlock) > maxRulesPayloadBytes {
		return fmt.Errorf("rules payload for %q is about %d bytes (limit %d): split rules, use smaller files, or pass --skip-size-check to override (not recommended)",
			objectName, len(rulesBlock), maxRulesPayloadBytes)
	}
	return nil
}

func checkSecretStringDataSize(secretName string, entries map[string]string, opts Options) error {
	if opts.SkipSizeCheck {
		return nil
	}
	total := 0
	for k, v := range entries {
		n := len(v)
		if n > maxRulesPayloadBytes {
			return fmt.Errorf("secret %q key %q is about %d bytes (limit %d per value): trim data files or pass --skip-size-check to override (not recommended)",
				secretName, k, n, maxRulesPayloadBytes)
		}
		total += n
	}
	if total > maxSecretStringDataTotalBytes {
		return fmt.Errorf("secret %q stringData total is about %d bytes (limit %d): split data across secrets or pass --skip-size-check to override (not recommended)",
			secretName, total, maxSecretStringDataTotalBytes)
	}
	return nil
}

// validateConfigMapObjectName ensures the final metadata.name is acceptable to the
// apiserver (RFC 1123 DNS subdomain, max 253 runes), including after NamePrefix/NameSuffix.
func validateConfigMapObjectName(name string) error {
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("invalid ConfigMap name %q: %s", name, strings.Join(errs, "; "))
	}
	return nil
}

func generateConfigMapName(fileBase string) (string, error) {
	name := strings.ToLower(strings.TrimSuffix(fileBase, ".conf"))
	name = strings.ReplaceAll(name, "_", "-")
	name = nameSanitizeRe.ReplaceAllString(name, "")
	name = strings.TrimLeft(name, "-.")
	name = strings.TrimRight(name, "-.")
	if name == "" {
		return "", fmt.Errorf("cannot generate valid ConfigMap name from file: %s", fileBase)
	}
	return name, nil
}
