package corerulesetgen

import (
	"fmt"
	"io"
)

// WriteManifests writes multi-document YAML in the same order as Generate: base ConfigMap,
// extra ConfigMaps, optional Secret, RuleSet (with trailing newline after RuleSet).
func WriteManifests(w io.Writer, b *ManifestBundle) error {
	if _, err := fmt.Fprintln(w, b.BaseConfigMapYAML); err != nil {
		return err
	}
	for _, cm := range b.ExtraConfigMaps {
		if _, err := fmt.Fprint(w, "---\n"+cm.Doc); err != nil {
			return err
		}
	}
	if b.SecretDoc != "" {
		if _, err := fmt.Fprint(w, "---\n"+b.SecretDoc); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(w, "---\n"+b.RuleSetDoc+"\n"); err != nil {
		return err
	}
	return nil
}
