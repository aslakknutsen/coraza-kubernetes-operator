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

package framework

import (
	"context"
	"fmt"
	"os"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// -----------------------------------------------------------------------------
// GVRs
// -----------------------------------------------------------------------------

var (
	// EngineGVR is the GroupVersionResource for Engine resources.
	EngineGVR = schema.GroupVersionResource{
		Group: "waf.k8s.coraza.io", Version: "v1alpha1", Resource: "engines",
	}

	// RuleSetGVR is the GroupVersionResource for RuleSet resources.
	RuleSetGVR = schema.GroupVersionResource{
		Group: "waf.k8s.coraza.io", Version: "v1alpha1", Resource: "rulesets",
	}

	// GatewayGVR is the GroupVersionResource for Gateway resources.
	GatewayGVR = schema.GroupVersionResource{
		Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways",
	}

	// WasmPluginGVR is the GroupVersionResource for WasmPlugin resources.
	WasmPluginGVR = schema.GroupVersionResource{
		Group: "extensions.istio.io", Version: "v1alpha1", Resource: "wasmplugins",
	}
)

// -----------------------------------------------------------------------------
// Option Types
// -----------------------------------------------------------------------------

// RuleRef is a reference to a rule source (e.g., a ConfigMap).
type RuleRef struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string // set for cross-namespace testing; leave empty for same-namespace
}

// EngineOpts configures an Engine resource for creation.
type EngineOpts struct {
	// RuleSetName is the name of the RuleSet to reference (required).
	RuleSetName string

	// RuleSetNamespace sets a cross-namespace reference. Leave empty for
	// same-namespace (the common case).
	RuleSetNamespace string

	// GatewayName sets the workload selector to target this gateway's pods
	// via the gateway.networking.k8s.io/gateway-name label. Ignored if
	// WorkloadLabels is set.
	GatewayName string

	// WorkloadLabels overrides the workload selector. Takes precedence over
	// GatewayName.
	WorkloadLabels map[string]string

	// WasmImage is the OCI image for the WASM plugin. Defaults to the
	// CORAZA_WASM_IMAGE env var, or a built-in default.
	WasmImage string

	// FailurePolicy is "fail" or "allow". Defaults to "fail".
	FailurePolicy string

	// PollInterval is the ruleSetCacheServer poll interval in seconds.
	// Defaults to 5.
	PollInterval int64
}

// -----------------------------------------------------------------------------
// Defaults
// -----------------------------------------------------------------------------

const fallbackWasmImage = "oci://ghcr.io/networking-incubator/coraza-proxy-wasm:179ea90b2617f557f805fe672daf880c14c6b8b7"

func defaultWasmImage() string {
	if img := os.Getenv("CORAZA_WASM_IMAGE"); img != "" {
		return img
	}
	return fallbackWasmImage
}

// SimpleBlockRule generates a SecLang rule that denies requests containing
// the target string with the given rule ID.
func SimpleBlockRule(id int, target string) string {
	return fmt.Sprintf(
		`SecRule ARGS|REQUEST_URI|REQUEST_HEADERS "@contains %s" "id:%d,phase:2,deny,status:403,msg:'%s blocked'"`,
		target, id, target,
	)
}

// -----------------------------------------------------------------------------
// Resource Builders (exported for direct use or testing)
// -----------------------------------------------------------------------------

// BuildGateway builds an unstructured Gateway object with Istio annotations.
func BuildGateway(namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "Gateway",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"istio.io/rev": "coraza",
				},
				"annotations": map[string]interface{}{
					"networking.istio.io/service-type": "ClusterIP",
				},
			},
			"spec": map[string]interface{}{
				"gatewayClassName": "istio",
				"listeners": []interface{}{
					map[string]interface{}{
						"name":     "http",
						"port":     int64(80),
						"protocol": "HTTP",
						"allowedRoutes": map[string]interface{}{
							"namespaces": map[string]interface{}{
								"from": "All",
							},
						},
					},
				},
			},
		},
	}
}

// BuildRuleSet builds an unstructured RuleSet object.
func BuildRuleSet(namespace, name string, rules []RuleRef) *unstructured.Unstructured {
	ruleList := make([]interface{}, len(rules))
	for i, r := range rules {
		rule := map[string]interface{}{
			"apiVersion": r.APIVersion,
			"kind":       r.Kind,
			"name":       r.Name,
		}
		if r.Namespace != "" {
			rule["namespace"] = r.Namespace
		}
		ruleList[i] = rule
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "waf.k8s.coraza.io/v1alpha1",
			"kind":       "RuleSet",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"rules": ruleList,
			},
		},
	}
}

// BuildEngine builds an unstructured Engine object.
func BuildEngine(namespace, name string, opts EngineOpts) *unstructured.Unstructured {
	if opts.WasmImage == "" {
		opts.WasmImage = defaultWasmImage()
	}
	if opts.FailurePolicy == "" {
		opts.FailurePolicy = "fail"
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = 5
	}

	workloadLabels := opts.WorkloadLabels
	if workloadLabels == nil && opts.GatewayName != "" {
		workloadLabels = map[string]string{
			"gateway.networking.k8s.io/gateway-name": opts.GatewayName,
		}
	}
	if workloadLabels == nil {
		workloadLabels = map[string]string{"app": "gateway"}
	}

	// Convert to map[string]interface{} for unstructured
	labels := make(map[string]interface{}, len(workloadLabels))
	for k, v := range workloadLabels {
		labels[k] = v
	}

	ruleSetRef := map[string]interface{}{
		"apiVersion": "waf.k8s.coraza.io/v1alpha1",
		"kind":       "RuleSet",
		"name":       opts.RuleSetName,
	}
	if opts.RuleSetNamespace != "" {
		ruleSetRef["namespace"] = opts.RuleSetNamespace
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "waf.k8s.coraza.io/v1alpha1",
			"kind":       "Engine",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"ruleSet":       ruleSetRef,
				"failurePolicy": opts.FailurePolicy,
				"driver": map[string]interface{}{
					"istio": map[string]interface{}{
						"wasm": map[string]interface{}{
							"image": opts.WasmImage,
							"mode":  "gateway",
							"workloadSelector": map[string]interface{}{
								"matchLabels": labels,
							},
							"ruleSetCacheServer": map[string]interface{}{
								"pollIntervalSeconds": opts.PollInterval,
							},
						},
					},
				},
			},
		},
	}
}

// -----------------------------------------------------------------------------
// Scenario - Resource Creation Methods
// -----------------------------------------------------------------------------

// CreateConfigMap creates a ConfigMap with WAF rules and registers cleanup.
func (s *Scenario) CreateConfigMap(namespace, name, rules string) {
	s.T.Helper()
	ctx := context.Background()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"rules": rules,
		},
	}
	_, err := s.F.KubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
	require.NoError(s.T, err, "create ConfigMap %s/%s", namespace, name)

	s.T.Logf("Created ConfigMap: %s/%s", namespace, name)
	s.OnCleanup(func() {
		if err := s.F.KubeClient.CoreV1().ConfigMaps(namespace).Delete(
			context.Background(), name, metav1.DeleteOptions{},
		); err != nil {
			s.T.Logf("cleanup: failed to delete ConfigMap %s/%s: %v", namespace, name, err)
		}
	})
}

// CreateGateway creates a Gateway resource and registers cleanup.
func (s *Scenario) CreateGateway(namespace, name string) {
	s.T.Helper()
	ctx := context.Background()

	obj := BuildGateway(namespace, name)
	_, err := s.F.DynamicClient.Resource(GatewayGVR).Namespace(namespace).Create(
		ctx, obj, metav1.CreateOptions{},
	)
	require.NoError(s.T, err, "create Gateway %s/%s", namespace, name)

	s.T.Logf("Created Gateway: %s/%s", namespace, name)
	s.OnCleanup(func() {
		if err := s.F.DynamicClient.Resource(GatewayGVR).Namespace(namespace).Delete(
			context.Background(), name, metav1.DeleteOptions{},
		); err != nil {
			s.T.Logf("cleanup: failed to delete Gateway %s/%s: %v", namespace, name, err)
		}
	})
}

// CreateRuleSet creates a RuleSet resource and registers cleanup. Fails the
// test on error. Use TryCreateRuleSet to get the error instead.
func (s *Scenario) CreateRuleSet(namespace, name string, rules []RuleRef) {
	s.T.Helper()
	err := s.TryCreateRuleSet(namespace, name, rules)
	require.NoError(s.T, err, "create RuleSet %s/%s", namespace, name)

	s.T.Logf("Created RuleSet: %s/%s", namespace, name)
	s.OnCleanup(func() {
		if err := s.F.DynamicClient.Resource(RuleSetGVR).Namespace(namespace).Delete(
			context.Background(), name, metav1.DeleteOptions{},
		); err != nil {
			s.T.Logf("cleanup: failed to delete RuleSet %s/%s: %v", namespace, name, err)
		}
	})
}

// TryCreateRuleSet attempts to create a RuleSet and returns any error.
// Use this when testing validation rejection.
func (s *Scenario) TryCreateRuleSet(namespace, name string, rules []RuleRef) error {
	obj := BuildRuleSet(namespace, name, rules)
	_, err := s.F.DynamicClient.Resource(RuleSetGVR).Namespace(namespace).Create(
		context.Background(), obj, metav1.CreateOptions{},
	)
	return err
}

// CreateEngine creates an Engine resource and registers cleanup. Fails the
// test on error. Use TryCreateEngine to get the error instead.
func (s *Scenario) CreateEngine(namespace, name string, opts EngineOpts) {
	s.T.Helper()
	err := s.TryCreateEngine(namespace, name, opts)
	require.NoError(s.T, err, "create Engine %s/%s", namespace, name)

	s.T.Logf("Created Engine: %s/%s", namespace, name)
	s.OnCleanup(func() {
		if err := s.F.DynamicClient.Resource(EngineGVR).Namespace(namespace).Delete(
			context.Background(), name, metav1.DeleteOptions{},
		); err != nil {
			s.T.Logf("cleanup: failed to delete Engine %s/%s: %v", namespace, name, err)
		}
	})
}

// TryCreateEngine attempts to create an Engine and returns any error.
// Use this when testing validation rejection.
func (s *Scenario) TryCreateEngine(namespace, name string, opts EngineOpts) error {
	obj := BuildEngine(namespace, name, opts)
	_, err := s.F.DynamicClient.Resource(EngineGVR).Namespace(namespace).Create(
		context.Background(), obj, metav1.CreateOptions{},
	)
	return err
}
