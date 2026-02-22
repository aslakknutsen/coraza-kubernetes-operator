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

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	wafv1alpha1 "github.com/networking-incubator/coraza-kubernetes-operator/api/v1alpha1"
	"github.com/networking-incubator/coraza-kubernetes-operator/test/utils"
)

// -----------------------------------------------------------------------------
// Gateway Status Tests - Gateway Resolution
// -----------------------------------------------------------------------------

func TestResolveTargetGateway(t *testing.T) {
	tests := []struct {
		name           string
		engine         *wafv1alpha1.Engine
		expectedGWName string
		expectedGWNS   string
	}{
		{
			name: "resolves gateway from workloadSelector matchLabels",
			engine: utils.NewTestEngine(utils.EngineOptions{
				Name:      "test-engine",
				Namespace: "default",
				WorkloadLabels: map[string]string{
					"gateway.networking.k8s.io/gateway-name": "my-gateway",
				},
			}),
			expectedGWName: "my-gateway",
			expectedGWNS:   "default",
		},
		{
			name: "returns empty when gateway label is missing",
			engine: utils.NewTestEngine(utils.EngineOptions{
				Name:      "test-engine",
				Namespace: "default",
				WorkloadLabels: map[string]string{
					"app": "something",
				},
			}),
			expectedGWName: "",
			expectedGWNS:   "",
		},
		{
			name: "returns empty when workloadSelector is nil",
			engine: func() *wafv1alpha1.Engine {
				e := utils.NewTestEngine(utils.EngineOptions{
					Name:      "test-engine",
					Namespace: "default",
				})
				e.Spec.Driver.Istio.Wasm.WorkloadSelector = nil
				return e
			}(),
			expectedGWName: "",
			expectedGWNS:   "",
		},
		{
			name: "returns empty when istio driver is nil",
			engine: func() *wafv1alpha1.Engine {
				e := utils.NewTestEngine(utils.EngineOptions{
					Name:      "test-engine",
					Namespace: "default",
				})
				e.Spec.Driver.Istio = nil
				return e
			}(),
			expectedGWName: "",
			expectedGWNS:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, ns := resolveTargetGateway(tt.engine)
			assert.Equal(t, tt.expectedGWName, name)
			assert.Equal(t, tt.expectedGWNS, ns)
		})
	}
}

func TestGatewayConditionType(t *testing.T) {
	assert.Equal(t, "waf.k8s.coraza.io/EngineReady-my-engine", gatewayConditionType("my-engine"))
	assert.Equal(t, "waf.k8s.coraza.io/EngineReady-other", gatewayConditionType("other"))
}

// -----------------------------------------------------------------------------
// Gateway Status Tests - Set/Remove Conditions (envtest)
// -----------------------------------------------------------------------------

func createTestGateway(t *testing.T, ctx context.Context, name, namespace string) *unstructured.Unstructured {
	t.Helper()
	gw := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "Gateway",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"gatewayClassName": "istio",
				"listeners": []interface{}{
					map[string]interface{}{
						"name":     "http",
						"port":     int64(80),
						"protocol": "HTTP",
					},
				},
			},
		},
	}
	gw.SetGroupVersionKind(gatewayGVK)
	err := k8sClient.Create(ctx, gw)
	require.NoError(t, err)
	return gw
}

func getGatewayConditions(t *testing.T, ctx context.Context, name, namespace string) []interface{} {
	t.Helper()
	gw := &unstructured.Unstructured{}
	gw.SetGroupVersionKind(gatewayGVK)
	err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, gw)
	require.NoError(t, err)
	conditions, _, _ := unstructured.NestedSlice(gw.Object, "status", "conditions")
	return conditions
}

func TestSetGatewayEngineCondition(t *testing.T) {
	ctx := context.Background()

	gwName := "test-gw-set-condition"
	gwNamespace := "default"
	gw := createTestGateway(t, ctx, gwName, gwNamespace)
	defer func() {
		_ = k8sClient.Delete(ctx, gw)
	}()

	reconciler := &EngineReconciler{
		Client:   k8sClient,
		Scheme:   scheme,
		Recorder: utils.NewTestRecorder(),
	}

	engine := utils.NewTestEngine(utils.EngineOptions{
		Name:      "my-engine",
		Namespace: gwNamespace,
	})

	err := reconciler.setGatewayEngineCondition(ctx, gwName, gwNamespace, engine, metav1.ConditionTrue, "EngineAttached", "Engine default/my-engine is ready")
	require.NoError(t, err)

	conditions := getGatewayConditions(t, ctx, gwName, gwNamespace)
	require.NotEmpty(t, conditions)

	found := false
	expectedType := gatewayConditionType("my-engine")
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == expectedType {
			found = true
			assert.Equal(t, "True", cond["status"])
			assert.Equal(t, "EngineAttached", cond["reason"])
			assert.Contains(t, cond["message"], "Engine default/my-engine is ready")
		}
	}
	assert.True(t, found, "expected to find condition type %s", expectedType)
}

func TestRemoveGatewayEngineCondition(t *testing.T) {
	ctx := context.Background()

	gwName := "test-gw-remove-condition"
	gwNamespace := "default"
	gw := createTestGateway(t, ctx, gwName, gwNamespace)
	defer func() {
		_ = k8sClient.Delete(ctx, gw)
	}()

	reconciler := &EngineReconciler{
		Client:   k8sClient,
		Scheme:   scheme,
		Recorder: utils.NewTestRecorder(),
	}

	engine := utils.NewTestEngine(utils.EngineOptions{
		Name:      "removable-engine",
		Namespace: gwNamespace,
	})

	err := reconciler.setGatewayEngineCondition(ctx, gwName, gwNamespace, engine, metav1.ConditionTrue, "EngineAttached", "ready")
	require.NoError(t, err)

	conditions := getGatewayConditions(t, ctx, gwName, gwNamespace)
	require.NotEmpty(t, conditions)

	err = reconciler.removeGatewayEngineCondition(ctx, gwName, gwNamespace, "removable-engine")
	require.NoError(t, err)

	conditions = getGatewayConditions(t, ctx, gwName, gwNamespace)
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		assert.NotEqual(t, gatewayConditionType("removable-engine"), cond["type"])
	}
}

func TestRemoveGatewayEngineCondition_GatewayNotFound(t *testing.T) {
	ctx := context.Background()

	reconciler := &EngineReconciler{
		Client:   k8sClient,
		Scheme:   scheme,
		Recorder: utils.NewTestRecorder(),
	}

	err := reconciler.removeGatewayEngineCondition(ctx, "nonexistent-gw", "default", "some-engine")
	assert.NoError(t, err, "should succeed when gateway doesn't exist")
}

// -----------------------------------------------------------------------------
// Gateway Status Tests - Finalizer Lifecycle
// -----------------------------------------------------------------------------

func TestEngineReconciler_FinalizerAddedOnReconcile(t *testing.T) {
	ctx := context.Background()

	gwName := "test-gw-finalizer"
	gwNamespace := "default"
	gw := createTestGateway(t, ctx, gwName, gwNamespace)
	defer func() {
		_ = k8sClient.Delete(ctx, gw)
	}()

	engine := utils.NewTestEngine(utils.EngineOptions{
		Name:      "finalizer-test-engine",
		Namespace: gwNamespace,
		WorkloadLabels: map[string]string{
			"gateway.networking.k8s.io/gateway-name": gwName,
		},
	})
	require.NoError(t, k8sClient.Create(ctx, engine))
	defer func() {
		_ = k8sClient.Delete(ctx, engine)
	}()

	reconciler := &EngineReconciler{
		Client:                    k8sClient,
		Scheme:                    scheme,
		Recorder:                  utils.NewTestRecorder(),
		ruleSetCacheServerCluster: "test-cluster",
	}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      engine.Name,
			Namespace: engine.Namespace,
		},
	})
	require.NoError(t, err)

	var updated wafv1alpha1.Engine
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: engine.Name, Namespace: engine.Namespace}, &updated))

	assert.Contains(t, updated.Finalizers, gatewayStatusCleanupFinalizer)
	assert.NotEmpty(t, updated.Status.TargetGateways)
	assert.Equal(t, gwName, updated.Status.TargetGateways[0].Name)
	assert.True(t, updated.Status.TargetGateways[0].Attached)
}

func TestEngineReconciler_GatewayConditionSetOnReconcile(t *testing.T) {
	ctx := context.Background()

	gwName := "test-gw-condition-reconcile"
	gwNamespace := "default"
	gw := createTestGateway(t, ctx, gwName, gwNamespace)
	defer func() {
		_ = k8sClient.Delete(ctx, gw)
	}()

	engine := utils.NewTestEngine(utils.EngineOptions{
		Name:      "gw-condition-test-engine",
		Namespace: gwNamespace,
		WorkloadLabels: map[string]string{
			"gateway.networking.k8s.io/gateway-name": gwName,
		},
	})
	require.NoError(t, k8sClient.Create(ctx, engine))
	defer func() {
		_ = k8sClient.Delete(ctx, engine)
	}()

	reconciler := &EngineReconciler{
		Client:                    k8sClient,
		Scheme:                    scheme,
		Recorder:                  utils.NewTestRecorder(),
		ruleSetCacheServerCluster: "test-cluster",
	}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      engine.Name,
			Namespace: engine.Namespace,
		},
	})
	require.NoError(t, err)

	conditions := getGatewayConditions(t, ctx, gwName, gwNamespace)
	require.NotEmpty(t, conditions)

	expectedType := gatewayConditionType(engine.Name)
	found := false
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == expectedType {
			found = true
			assert.Equal(t, "True", cond["status"])
			assert.Equal(t, "EngineAttached", cond["reason"])
		}
	}
	assert.True(t, found, "expected Gateway to have condition %s", expectedType)
}

func TestEngineReconciler_MultipleEnginesSameGateway(t *testing.T) {
	ctx := context.Background()

	gwName := "test-gw-multi-engine"
	gwNamespace := "default"
	gw := createTestGateway(t, ctx, gwName, gwNamespace)
	defer func() {
		_ = k8sClient.Delete(ctx, gw)
	}()

	reconciler := &EngineReconciler{
		Client:                    k8sClient,
		Scheme:                    scheme,
		Recorder:                  utils.NewTestRecorder(),
		ruleSetCacheServerCluster: "test-cluster",
	}

	for _, engineName := range []string{"engine-alpha", "engine-beta"} {
		engine := utils.NewTestEngine(utils.EngineOptions{
			Name:      engineName,
			Namespace: gwNamespace,
			WorkloadLabels: map[string]string{
				"gateway.networking.k8s.io/gateway-name": gwName,
			},
		})
		require.NoError(t, k8sClient.Create(ctx, engine))
		defer func() {
			_ = k8sClient.Delete(ctx, engine)
		}()

		_, err := reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      engineName,
				Namespace: gwNamespace,
			},
		})
		require.NoError(t, err)
	}

	conditions := getGatewayConditions(t, ctx, gwName, gwNamespace)
	condTypes := make(map[string]bool)
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if ct, ok := cond["type"].(string); ok {
			condTypes[ct] = true
		}
	}

	assert.True(t, condTypes[gatewayConditionType("engine-alpha")], "expected condition for engine-alpha")
	assert.True(t, condTypes[gatewayConditionType("engine-beta")], "expected condition for engine-beta")
}

func TestEngineReconciler_NoGatewayLabel(t *testing.T) {
	ctx := context.Background()

	engine := utils.NewTestEngine(utils.EngineOptions{
		Name:      "no-gw-label-engine",
		Namespace: "default",
		WorkloadLabels: map[string]string{
			"app": "something-else",
		},
	})
	require.NoError(t, k8sClient.Create(ctx, engine))
	defer func() {
		_ = k8sClient.Delete(ctx, engine)
	}()

	reconciler := &EngineReconciler{
		Client:                    k8sClient,
		Scheme:                    scheme,
		Recorder:                  utils.NewTestRecorder(),
		ruleSetCacheServerCluster: "test-cluster",
	}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      engine.Name,
			Namespace: engine.Namespace,
		},
	})
	require.NoError(t, err)

	var updated wafv1alpha1.Engine
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: engine.Name, Namespace: engine.Namespace}, &updated))

	assert.NotContains(t, updated.Finalizers, gatewayStatusCleanupFinalizer)
	assert.Empty(t, updated.Status.TargetGateways)
}
