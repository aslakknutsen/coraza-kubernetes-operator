//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	wafv1alpha1 "github.com/networking-incubator/coraza-kubernetes-operator/api/v1alpha1"
	"github.com/networking-incubator/coraza-kubernetes-operator/test/framework"
)

// ---------------------------------------------------------------------------
// Engine lifecycle edge cases
// ---------------------------------------------------------------------------

// TestEngineDeleteCleansUpWasmPlugin verifies that deleting an Engine removes
// the associated WasmPlugin (no orphans left behind).
func TestEngineDeleteCleansUpWasmPlugin(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-engine-del")

	s.Step("create rules and engine")
	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateConfigMap(ns, "block-rules", framework.SimpleBlockRule(90001, "advdel"))
	s.CreateRuleSet(ns, "ruleset", []string{"base-rules", "block-rules"})

	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.CreateEngine(ns, "engine", framework.EngineOpts{
		RuleSetName: "ruleset",
		GatewayName: "gw",
	})
	s.ExpectEngineReady(ns, "engine")
	s.ExpectWasmPluginExists(ns, "coraza-engine-engine")

	s.Step("delete the engine")
	err := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Delete(
		context.Background(), "engine", metav1.DeleteOptions{},
	)
	require.NoError(t, err, "delete engine")

	s.Step("verify WasmPlugin is cleaned up")
	s.ExpectResourceGone(ns, "coraza-engine-engine", framework.WasmPluginGVR)

	s.Step("verify no orphaned WasmPlugins remain")
	wps, err := s.F.DynamicClient.Resource(framework.WasmPluginGVR).Namespace(ns).List(
		context.Background(), metav1.ListOptions{},
	)
	require.NoError(t, err)
	assert.Empty(t, wps.Items, "expected no WasmPlugins after Engine deletion, found %d", len(wps.Items))
}

// TestDeleteRuleSetWhileEngineRunning verifies that deleting a RuleSet
// referenced by a running Engine causes the Engine to become Degraded,
// and traffic still flows (cached rules persist).
func TestDeleteRuleSetWhileEngineRunning(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-rs-del")

	s.Step("set up gateway + rules + engine")
	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateConfigMap(ns, "block-rules", framework.SimpleBlockRule(90010, "delbomb"))
	s.CreateRuleSet(ns, "ruleset", []string{"base-rules", "block-rules"})
	s.ExpectRuleSetReady(ns, "ruleset")

	s.CreateEngine(ns, "engine", framework.EngineOpts{
		RuleSetName: "ruleset",
		GatewayName: "gw",
	})
	s.ExpectEngineReady(ns, "engine")

	s.CreateEchoBackend(ns, "echo")
	s.CreateHTTPRoute(ns, "route", "gw", "echo")

	gw := s.ProxyToGateway(ns, "gw")
	gw.ExpectBlocked("/?q=delbomb")
	gw.ExpectAllowed("/?q=safe")

	s.Step("delete the RuleSet")
	err := s.F.DynamicClient.Resource(framework.RuleSetGVR).Namespace(ns).Delete(
		context.Background(), "ruleset", metav1.DeleteOptions{},
	)
	require.NoError(t, err)

	s.Step("verify engine becomes Degraded")
	s.ExpectEngineDegraded(ns, "engine")

	s.Step("verify cached rules still block traffic")
	gw.ExpectBlocked("/?q=delbomb")
	gw.ExpectAllowed("/?q=safe")
}

// TestEngineWithEmptyRuleSetRefs verifies CRD validation rejects a
// RuleSet with zero ConfigMap refs (spec.rules minItems=1).
func TestEngineWithEmptyRuleSetRefs(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-empty-rs")

	s.Step("try to create a RuleSet with zero rules refs")
	s.ExpectCreateFails("spec.rules", func() error {
		return s.TryCreateRuleSet(ns, "empty-ruleset", []string{})
	})
}

// TestSwitchEngineRuleSet verifies that updating an Engine's spec.ruleSet.name
// to point to a different RuleSet replaces the WasmPlugin with new rules.
func TestSwitchEngineRuleSet(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-switch-rs")

	s.Step("create gateway and two rulesets")
	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateConfigMap(ns, "rules-a", framework.SimpleBlockRule(90020, "alpha"))
	s.CreateConfigMap(ns, "rules-b", framework.SimpleBlockRule(90021, "bravo"))
	s.CreateRuleSet(ns, "ruleset-a", []string{"base-rules", "rules-a"})
	s.CreateRuleSet(ns, "ruleset-b", []string{"base-rules", "rules-b"})

	s.CreateEngine(ns, "engine", framework.EngineOpts{
		RuleSetName: "ruleset-a",
		GatewayName: "gw",
	})
	s.ExpectEngineReady(ns, "engine")

	s.CreateEchoBackend(ns, "echo")
	s.CreateHTTPRoute(ns, "route", "gw", "echo")

	gw := s.ProxyToGateway(ns, "gw")
	gw.ExpectBlocked("/?q=alpha")
	gw.ExpectAllowed("/?q=bravo")

	s.Step("switch engine to ruleset-b")
	obj, err := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Get(
		t.Context(), "engine", metav1.GetOptions{},
	)
	require.NoError(t, err)

	err = unstructured.SetNestedField(obj.Object, "ruleset-b", "spec", "ruleSet", "name")
	require.NoError(t, err)

	_, err = s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Update(
		t.Context(), obj, metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	s.Step("verify engine re-reconciles with new ruleset")
	s.ExpectEngineReady(ns, "engine")

	s.Step("verify new rules apply and old rules no longer block")
	gw.ExpectBlocked("/?q=bravo")
	gw.ExpectAllowed("/?q=alpha")
}

// ---------------------------------------------------------------------------
// RuleSet edge cases
// ---------------------------------------------------------------------------

// TestRuleSetReferencingNonExistentConfigMap verifies that a RuleSet
// referencing a ConfigMap that doesn't exist becomes Degraded.
func TestRuleSetReferencingNonExistentConfigMap(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-missing-cm")

	s.Step("create RuleSet referencing non-existent ConfigMap")
	s.CreateRuleSet(ns, "ruleset", []string{"does-not-exist"})

	s.Step("verify RuleSet becomes Degraded")
	s.ExpectRuleSetDegraded(ns, "ruleset")
}

// TestConfigMapWithEmptyRulesKey verifies behavior when a ConfigMap has
// an empty "rules" key.
func TestConfigMapWithEmptyRulesKey(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-empty-cm")

	s.Step("create ConfigMap with empty rules string")
	s.CreateConfigMap(ns, "empty-rules", "")

	s.Step("create RuleSet referencing empty ConfigMap")
	s.CreateRuleSet(ns, "ruleset", []string{"empty-rules"})

	s.Step("check if RuleSet handles empty content")
	// Empty rules should either cause Degraded (no valid SecRuleEngine)
	// or Ready (no rules to compile is a valid state).
	// Wait and see which condition appears.
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		obj, err := s.F.DynamicClient.Resource(framework.RuleSetGVR).Namespace(ns).Get(
			t.Context(), "ruleset", metav1.GetOptions{},
		)
		if !assert.NoError(collect, err) {
			return
		}
		conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
		assert.NotEmpty(collect, conditions, "RuleSet should have at least one condition")
	}, framework.DefaultTimeout, framework.DefaultInterval)
}

// TestDeleteConfigMapReferencedByReadyRuleSet verifies that deleting a
// ConfigMap referenced by a Ready RuleSet causes it to become Degraded.
func TestDeleteConfigMapReferencedByReadyRuleSet(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-cm-gone")

	s.Step("create ConfigMap and RuleSet")
	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateConfigMap(ns, "block-rules", framework.SimpleBlockRule(90030, "gone"))
	s.CreateRuleSet(ns, "ruleset", []string{"base-rules", "block-rules"})
	s.ExpectRuleSetReady(ns, "ruleset")

	s.Step("delete ConfigMap referenced by the RuleSet")
	err := s.F.KubeClient.CoreV1().ConfigMaps(ns).Delete(
		context.Background(), "block-rules", metav1.DeleteOptions{},
	)
	require.NoError(t, err)

	s.Step("verify RuleSet detects the missing ConfigMap and becomes Degraded")
	s.ExpectRuleSetDegraded(ns, "ruleset")
}

// ---------------------------------------------------------------------------
// Gateway and routing edge cases
// ---------------------------------------------------------------------------

// TestDeleteGatewayUpdatesEngineStatus verifies that deleting a Gateway
// causes the Engine's status.gateways to update (remove the deleted gw).
func TestDeleteGatewayUpdatesEngineStatus(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-gw-del")

	s.Step("create gateway and engine")
	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateRuleSet(ns, "ruleset", []string{"base-rules"})

	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.CreateEngine(ns, "engine", framework.EngineOpts{
		RuleSetName: "ruleset",
		GatewayName: "gw",
	})
	s.ExpectEngineReady(ns, "engine")
	s.ExpectEngineGateways(ns, "engine", []string{"gw"})

	s.Step("delete the gateway")
	err := s.F.DynamicClient.Resource(framework.GatewayGVR).Namespace(ns).Delete(
		context.Background(), "gw", metav1.DeleteOptions{},
	)
	require.NoError(t, err)

	s.Step("verify engine status.gateways becomes empty")
	s.ExpectEngineGateways(ns, "engine", nil)
}

// TestGatewayDeleteAndRecreate verifies that deleting and recreating a
// Gateway with the same name causes the Engine to re-attach.
func TestGatewayDeleteAndRecreate(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-gw-recreate")

	s.Step("set up gateway + rules + engine")
	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateConfigMap(ns, "block-rules", framework.SimpleBlockRule(90040, "recreated"))
	s.CreateRuleSet(ns, "ruleset", []string{"base-rules", "block-rules"})

	s.CreateEngine(ns, "engine", framework.EngineOpts{
		RuleSetName: "ruleset",
		GatewayName: "gw",
	})
	s.ExpectEngineReady(ns, "engine")
	s.ExpectEngineGateways(ns, "engine", []string{"gw"})

	s.Step("delete gateway")
	err := s.F.DynamicClient.Resource(framework.GatewayGVR).Namespace(ns).Delete(
		context.Background(), "gw", metav1.DeleteOptions{},
	)
	require.NoError(t, err)
	s.ExpectResourceGone(ns, "gw", framework.GatewayGVR)

	s.Step("wait for engine to notice gateway is gone")
	s.ExpectEngineGateways(ns, "engine", nil)

	s.Step("recreate gateway with same name")
	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.Step("verify engine re-attaches to the new gateway")
	s.ExpectEngineGateways(ns, "engine", []string{"gw"})

	s.Step("verify WAF enforcement on the new gateway")
	s.CreateEchoBackend(ns, "echo")
	s.CreateHTTPRoute(ns, "route", "gw", "echo")

	gw := s.ProxyToGateway(ns, "gw")
	gw.ExpectBlocked("/?q=recreated")
	gw.ExpectAllowed("/?q=safe")
}

// ---------------------------------------------------------------------------
// WAF behavior under mutation
// ---------------------------------------------------------------------------

// TestRuleMutationDenyToPass verifies that changing a ConfigMap rule action
// from "deny" to "pass" while traffic is flowing takes effect.
func TestRuleMutationDenyToPass(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-deny-pass")

	s.Step("create gateway + rules that deny 'target'")
	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateConfigMap(ns, "action-rules",
		`SecRule ARGS|REQUEST_URI "@contains target" "id:90050,phase:2,deny,status:403,msg:'target blocked'"`,
	)
	s.CreateRuleSet(ns, "ruleset", []string{"base-rules", "action-rules"})

	s.CreateEngine(ns, "engine", framework.EngineOpts{
		RuleSetName: "ruleset",
		GatewayName: "gw",
	})
	s.ExpectEngineReady(ns, "engine")

	s.CreateEchoBackend(ns, "echo")
	s.CreateHTTPRoute(ns, "route", "gw", "echo")

	gw := s.ProxyToGateway(ns, "gw")
	gw.ExpectBlocked("/?q=target")

	s.Step("change rule action from deny to pass")
	s.UpdateConfigMap(ns, "action-rules",
		`SecRule ARGS|REQUEST_URI "@contains target" "id:90050,phase:2,pass,msg:'target allowed'"`,
	)

	s.Step("verify previously blocked request now passes")
	gw.ExpectAllowed("/?q=target")
}

// TestSwitchFailurePolicy verifies that changing failurePolicy from
// "fail" to "allow" on a running Engine takes effect.
func TestSwitchFailurePolicy(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-fp-switch")

	s.Step("create engine with failurePolicy=fail referencing missing ruleset")
	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.CreateEngine(ns, "engine", framework.EngineOpts{
		RuleSetName:   "nonexistent",
		GatewayName:   "gw",
		FailurePolicy: wafv1alpha1.FailurePolicyFail,
	})
	s.ExpectEngineDegraded(ns, "engine")

	s.Step("switch failurePolicy to allow")
	obj, err := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Get(
		t.Context(), "engine", metav1.GetOptions{},
	)
	require.NoError(t, err)

	err = unstructured.SetNestedField(obj.Object, "allow", "spec", "failurePolicy")
	require.NoError(t, err)

	_, err = s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Update(
		t.Context(), obj, metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	s.Step("verify the update was accepted")
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		updated, getErr := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Get(
			t.Context(), "engine", metav1.GetOptions{},
		)
		if !assert.NoError(collect, getErr) {
			return
		}
		fp, _, _ := unstructured.NestedString(updated.Object, "spec", "failurePolicy")
		assert.Equal(collect, "allow", fp, "failurePolicy should be 'allow' after update")
	}, framework.DefaultTimeout, framework.DefaultInterval)
}

// ---------------------------------------------------------------------------
// CRD validation
// ---------------------------------------------------------------------------

// TestCRDValidationInvalidFailurePolicy verifies the API server rejects
// an Engine with an invalid failurePolicy value.
func TestCRDValidationInvalidFailurePolicy(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-val-fp")

	s.Step("try to create engine with invalid failurePolicy")
	obj := framework.BuildEngine(ns, "bad-engine", framework.EngineOpts{
		RuleSetName: "ruleset",
		GatewayName: "gw",
	})
	err := unstructured.SetNestedField(obj.Object, "explode", "spec", "failurePolicy")
	require.NoError(t, err)

	_, createErr := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Create(
		t.Context(), obj, metav1.CreateOptions{},
	)
	require.Error(t, createErr, "expected API server to reject invalid failurePolicy")
	require.Contains(t, createErr.Error(), "failurePolicy",
		"error should mention failurePolicy, got: %v", createErr)
}

// TestCRDValidationNegativePollInterval verifies the API server rejects
// an Engine with a negative pollIntervalSeconds.
func TestCRDValidationNegativePollInterval(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-val-poll")

	s.Step("try to create engine with negative poll interval")
	obj := framework.BuildEngine(ns, "bad-engine", framework.EngineOpts{
		RuleSetName:  "ruleset",
		GatewayName:  "gw",
		PollInterval: -1,
	})

	_, createErr := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Create(
		t.Context(), obj, metav1.CreateOptions{},
	)
	require.Error(t, createErr, "expected API server to reject negative pollIntervalSeconds")
}

// TestCRDValidationZeroPollInterval verifies the API server rejects
// an Engine with pollIntervalSeconds=0.
func TestCRDValidationZeroPollInterval(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-val-poll0")

	s.Step("try to create engine with poll interval = 0")
	obj := framework.BuildEngine(ns, "bad-engine", framework.EngineOpts{
		RuleSetName: "ruleset",
		GatewayName: "gw",
	})
	// BuildEngine defaults PollInterval to 5; override to 0
	err := unstructured.SetNestedField(obj.Object, int64(0),
		"spec", "driver", "istio", "wasm", "ruleSetCacheServer", "pollIntervalSeconds")
	require.NoError(t, err)

	_, createErr := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Create(
		t.Context(), obj, metav1.CreateOptions{},
	)
	require.Error(t, createErr, "expected API server to reject pollIntervalSeconds=0")
}

// TestCRDValidationEmptyRuleSetName verifies the API server rejects
// an Engine with an empty ruleSet.name.
func TestCRDValidationEmptyRuleSetName(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-val-rsname")

	s.Step("try to create engine with empty ruleSet name")
	obj := framework.BuildEngine(ns, "bad-engine", framework.EngineOpts{
		RuleSetName: "placeholder",
		GatewayName: "gw",
	})
	err := unstructured.SetNestedField(obj.Object, "", "spec", "ruleSet", "name")
	require.NoError(t, err)

	_, createErr := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Create(
		t.Context(), obj, metav1.CreateOptions{},
	)
	require.Error(t, createErr, "expected API server to reject empty ruleSet.name")
}

// TestCRDValidationMissingDriver verifies the API server rejects
// an Engine with no driver specified.
func TestCRDValidationMissingDriver(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-val-driver")

	s.Step("try to create engine without driver")
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "waf.k8s.coraza.io/v1alpha1",
			"kind":       "Engine",
			"metadata": map[string]interface{}{
				"name":      "no-driver",
				"namespace": ns,
			},
			"spec": map[string]interface{}{
				"ruleSet": map[string]interface{}{
					"name": "some-ruleset",
				},
			},
		},
	}

	_, createErr := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Create(
		t.Context(), obj, metav1.CreateOptions{},
	)
	require.Error(t, createErr, "expected API server to reject Engine without driver")
	require.Contains(t, createErr.Error(), "driver",
		"error should mention driver, got: %v", createErr)
}

// ---------------------------------------------------------------------------
// Operator events
// ---------------------------------------------------------------------------

// TestWarningEventOnMissingConfigMap verifies that a Warning event is
// emitted when a RuleSet references a ConfigMap that doesn't exist.
func TestWarningEventOnMissingConfigMap(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-warn-evt")

	s.Step("create RuleSet referencing non-existent ConfigMap")
	s.CreateRuleSet(ns, "ruleset", []string{"ghost-configmap"})
	s.ExpectRuleSetDegraded(ns, "ruleset")

	s.Step("verify Warning event was emitted")
	s.ExpectEvent(ns, framework.EventMatch{Type: "Warning"})
}

// TestNoSpuriousWarningsOnHappyPath verifies that no Warning events
// are emitted during a normal successful deployment.
func TestNoSpuriousWarningsOnHappyPath(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-no-warn")

	s.Step("create a complete working setup")
	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateConfigMap(ns, "block-rules", framework.SimpleBlockRule(90060, "noisy"))
	s.CreateRuleSet(ns, "ruleset", []string{"base-rules", "block-rules"})
	s.ExpectRuleSetReady(ns, "ruleset")

	s.CreateEngine(ns, "engine", framework.EngineOpts{
		RuleSetName: "ruleset",
		GatewayName: "gw",
	})
	s.ExpectEngineReady(ns, "engine")

	s.Step("verify Normal events exist")
	s.ExpectEvent(ns, framework.EventMatch{Type: "Normal", Reason: "RulesCached"})

	s.Step("verify no Warning events for our resources")
	events := s.GetEvents(ns)
	for _, e := range events {
		if e.Type == "Warning" {
			// Only fail if the Warning is about our operator resources
			regarding := ""
			if e.Regarding.Kind != "" {
				regarding = e.Regarding.Kind + "/" + e.Regarding.Name
			}
			if e.Regarding.Kind == "Engine" || e.Regarding.Kind == "RuleSet" {
				t.Errorf("unexpected Warning event for %s: reason=%s note=%s",
					regarding, e.Reason, e.Note)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Timing and ordering
// ---------------------------------------------------------------------------

// TestOutOfOrderCreation verifies eventual consistency when resources are
// created in reverse dependency order: Engine → RuleSet → ConfigMap → Gateway.
func TestOutOfOrderCreation(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-order")

	s.Step("create Engine first (no RuleSet, no Gateway)")
	s.CreateEngine(ns, "engine", framework.EngineOpts{
		RuleSetName: "ruleset",
		GatewayName: "gw",
	})

	s.Step("create RuleSet referencing non-existent ConfigMap")
	s.CreateRuleSet(ns, "ruleset", []string{"base-rules", "block-rules"})

	s.Step("create ConfigMaps")
	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateConfigMap(ns, "block-rules", framework.SimpleBlockRule(90070, "ordered"))

	s.Step("wait for RuleSet to become Ready")
	s.ExpectRuleSetReady(ns, "ruleset")

	s.Step("create Gateway last")
	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.Step("verify Engine eventually becomes Ready")
	s.ExpectEngineReady(ns, "engine")
	s.ExpectEngineGateways(ns, "engine", []string{"gw"})

	s.Step("verify WAF enforcement works")
	s.CreateEchoBackend(ns, "echo")
	s.CreateHTTPRoute(ns, "route", "gw", "echo")

	gw := s.ProxyToGateway(ns, "gw")
	gw.ExpectBlocked("/?q=ordered")
	gw.ExpectAllowed("/?q=safe")
}

// TestRapidEngineCreateDelete demonstrates a bug: after rapid Engine
// create/delete cycles the WASM plugin's rule matching becomes corrupted
// and blocks ALL traffic, including requests that should be allowed.
//
// Bug: the WAF rule `@contains rapid` matches `/?q=safe` after rapid
// Engine lifecycle cycles. The cache server returns 504 during rapid churn,
// and subsequent rule loading corrupts the rule matching state.
func TestRapidEngineCreateDelete(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-rapid-eng")

	s.Step("create shared rules")
	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateConfigMap(ns, "block-rules", framework.SimpleBlockRule(90080, "rapid"))
	s.CreateRuleSet(ns, "ruleset", []string{"base-rules", "block-rules"})
	s.ExpectRuleSetReady(ns, "ruleset")

	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	s.Step("rapid create/delete 5 times")
	for i := 0; i < 5; i++ {
		s.CreateEngine(ns, "engine", framework.EngineOpts{
			RuleSetName: "ruleset",
			GatewayName: "gw",
		})
		time.Sleep(500 * time.Millisecond)

		err := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Delete(
			context.Background(), "engine", metav1.DeleteOptions{},
		)
		require.NoError(t, err, "delete engine iteration %d", i)
		s.ExpectResourceGone(ns, "engine", framework.EngineGVR)
	}

	s.Step("create final engine and verify it works")
	s.CreateEngine(ns, "engine", framework.EngineOpts{
		RuleSetName: "ruleset",
		GatewayName: "gw",
	})
	s.ExpectEngineReady(ns, "engine")
	s.ExpectWasmPluginExists(ns, "coraza-engine-engine")

	s.CreateEchoBackend(ns, "echo")
	s.CreateHTTPRoute(ns, "route", "gw", "echo")

	gw := s.ProxyToGateway(ns, "gw")

	s.Step("verify blocking still works")
	gw.ExpectBlocked("/?q=rapid")

	s.Step("BUG: safe request is incorrectly blocked after rapid cycles")
	// After rapid create/delete cycles, the WAF blocks ALL traffic.
	// The rule `@contains rapid` incorrectly matches `/?q=safe`.
	gw.ExpectAllowed("/?q=safe")

	s.Step("verify no orphaned WasmPlugins")
	wps, err := s.F.DynamicClient.Resource(framework.WasmPluginGVR).Namespace(ns).List(
		context.Background(), metav1.ListOptions{},
	)
	require.NoError(t, err)
	assert.Len(t, wps.Items, 1, "expected exactly 1 WasmPlugin after rapid cycles")
}

// ---------------------------------------------------------------------------
// Resource cleanup
// ---------------------------------------------------------------------------

// TestWasmPluginOrphanCheck verifies that after cleaning up all Engines in a
// namespace, no WasmPlugins are left behind.
func TestWasmPluginOrphanCheck(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-orphan")

	s.Step("create multiple engines")
	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateRuleSet(ns, "ruleset", []string{"base-rules"})

	s.CreateGateway(ns, "gw")
	s.ExpectGatewayProgrammed(ns, "gw")

	for _, name := range []string{"engine-a", "engine-b", "engine-c"} {
		s.CreateEngine(ns, name, framework.EngineOpts{
			RuleSetName: "ruleset",
			GatewayName: "gw",
		})
		s.ExpectEngineReady(ns, name)
	}

	s.Step("verify 3 WasmPlugins exist")
	wps, err := s.F.DynamicClient.Resource(framework.WasmPluginGVR).Namespace(ns).List(
		context.Background(), metav1.ListOptions{},
	)
	require.NoError(t, err)
	assert.Len(t, wps.Items, 3, "expected 3 WasmPlugins for 3 engines")

	s.Step("delete all engines")
	for _, name := range []string{"engine-a", "engine-b", "engine-c"} {
		err := s.F.DynamicClient.Resource(framework.EngineGVR).Namespace(ns).Delete(
			context.Background(), name, metav1.DeleteOptions{},
		)
		require.NoError(t, err)
	}

	s.Step("verify all WasmPlugins are cleaned up")
	require.Eventually(t, func() bool {
		wps, err := s.F.DynamicClient.Resource(framework.WasmPluginGVR).Namespace(ns).List(
			context.Background(), metav1.ListOptions{},
		)
		if err != nil {
			return false
		}
		return len(wps.Items) == 0
	}, framework.DefaultTimeout, framework.DefaultInterval,
		"expected all WasmPlugins to be deleted after Engine cleanup")
}

// TestDuplicateConfigMapRefsInRuleSet verifies behavior when a RuleSet
// references the same ConfigMap twice.
func TestDuplicateConfigMapRefsInRuleSet(t *testing.T) {
	t.Parallel()
	s := fw.NewScenario(t)
	ns := s.GenerateNamespace("adv-dup-cm")

	s.Step("create ConfigMap and RuleSet with duplicate refs")
	s.CreateConfigMap(ns, "base-rules", `SecRuleEngine On`)
	s.CreateConfigMap(ns, "block-rules", framework.SimpleBlockRule(90090, "duped"))

	s.CreateRuleSet(ns, "ruleset", []string{"base-rules", "block-rules", "block-rules"})

	s.Step("check if RuleSet reaches Ready or Degraded")
	// Duplicate refs might cause duplicate rule IDs, which could cause
	// a compilation error, or the operator might deduplicate them.
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		obj, err := s.F.DynamicClient.Resource(framework.RuleSetGVR).Namespace(ns).Get(
			t.Context(), "ruleset", metav1.GetOptions{},
		)
		if !assert.NoError(collect, err) {
			return
		}
		conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
		assert.NotEmpty(collect, conditions, "RuleSet should have conditions")
	}, framework.DefaultTimeout, framework.DefaultInterval)

	obj, err := s.F.DynamicClient.Resource(framework.RuleSetGVR).Namespace(ns).Get(
		t.Context(), "ruleset", metav1.GetOptions{},
	)
	require.NoError(t, err)

	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	t.Logf("RuleSet with duplicate refs has conditions: %v", conditions)
}
