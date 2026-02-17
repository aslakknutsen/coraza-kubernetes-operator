//go:build integration

/*
Copyright 2026 Shane Utt.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"testing"

	"github.com/networking-incubator/coraza-kubernetes-operator/test/framework"
)

// TestCrossNamespaceRejection validates that cross-namespace references
// between Engines, RuleSets, and ConfigMaps are properly rejected by CRD
// validation.
//
// Cross-namespace support may be added in a future version, but for now
// all references must be within the same namespace.
//
// Related: https://github.com/networking-incubator/coraza-kubernetes-operator/issues/14
func TestCrossNamespaceRejection(t *testing.T) {
	s := fw.NewScenario(t)
	defer s.Cleanup()

	s.CreateNamespace("cross-ns-a")
	s.CreateNamespace("cross-ns-b")

	// -------------------------------------------------------------------------
	// Step 1: Set up resources in namespace A
	// -------------------------------------------------------------------------

	s.Step("create resources in namespace A")

	s.CreateConfigMap("cross-ns-a", "rules", `SecRuleEngine On`)
	s.CreateRuleSet("cross-ns-a", "ruleset-a", []framework.RuleRef{
		{APIVersion: "v1", Kind: "ConfigMap", Name: "rules"},
	})
	s.CreateGateway("cross-ns-a", "gateway-a")
	s.ExpectGatewayProgrammed("cross-ns-a", "gateway-a")

	// -------------------------------------------------------------------------
	// Step 2: Verify RuleSet rejects cross-namespace ConfigMap references
	// -------------------------------------------------------------------------

	s.Step("reject RuleSet with cross-namespace ConfigMap reference")

	s.ExpectCreateFails(
		"cross-namespace references are not currently supported",
		func() error {
			return s.TryCreateRuleSet("cross-ns-b", "cross-ref-ruleset", []framework.RuleRef{
				{APIVersion: "v1", Kind: "ConfigMap", Name: "rules", Namespace: "cross-ns-a"},
			})
		},
	)

	// -------------------------------------------------------------------------
	// Step 3: Verify Engine rejects cross-namespace RuleSet references
	// -------------------------------------------------------------------------

	s.Step("reject Engine with cross-namespace RuleSet reference")

	s.ExpectCreateFails(
		"cross-namespace references are not currently supported",
		func() error {
			return s.TryCreateEngine("cross-ns-b", "cross-ref-engine", framework.EngineOpts{
				RuleSetName:      "ruleset-a",
				RuleSetNamespace: "cross-ns-a",
				GatewayName:      "gateway-a",
			})
		},
	)

	// -------------------------------------------------------------------------
	// Step 4: Verify same-namespace references work correctly
	// -------------------------------------------------------------------------

	s.Step("verify same-namespace references succeed")

	s.CreateConfigMap("cross-ns-b", "local-rules", `SecRuleEngine On`)
	s.CreateRuleSet("cross-ns-b", "local-ruleset", []framework.RuleRef{
		{APIVersion: "v1", Kind: "ConfigMap", Name: "local-rules"},
	})
	s.CreateGateway("cross-ns-b", "gateway-b")
	s.ExpectGatewayProgrammed("cross-ns-b", "gateway-b")

	s.CreateEngine("cross-ns-b", "local-engine", framework.EngineOpts{
		RuleSetName: "local-ruleset",
		GatewayName: "gateway-b",
	})
	s.ExpectEngineReady("cross-ns-b", "local-engine")
}
