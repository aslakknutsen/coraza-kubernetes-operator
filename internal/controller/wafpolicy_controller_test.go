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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	wafv1alpha1 "github.com/networking-incubator/coraza-kubernetes-operator/api/v1alpha1"
	"github.com/networking-incubator/coraza-kubernetes-operator/test/utils"
)

func newTestWAFPolicyReconciler() *WAFPolicyReconciler {
	return &WAFPolicyReconciler{
		Client:   k8sClient,
		Scheme:   scheme,
		Recorder: utils.NewTestRecorder(),
		Config: WAFPolicyTranslatorConfig{
			DefaultWasmImage:    "oci://ghcr.io/test/coraza-proxy-wasm:latest",
			DefaultPollInterval: 15,
			EnvoyClusterName:    "test-cluster",
		},
	}
}

func TestWAFPolicyReconciler_ReconcileNotFound(t *testing.T) {
	reconciler := newTestWAFPolicyReconciler()

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent",
			Namespace: "default",
		},
	})

	require.NoError(t, err)
	assert.False(t, result.Requeue)
}

func TestWAFPolicyReconciler_ReconcileGatewayNotFound(t *testing.T) {
	ctx := context.Background()

	policy := &wafv1alpha1.WAFPolicy{}
	policy.Name = "test-gw-notfound"
	policy.Namespace = "default"
	policy.Spec = wafv1alpha1.WAFPolicySpec{
		TargetRef: gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
			LocalPolicyTargetReference: gwapiv1.LocalPolicyTargetReference{
				Group: "gateway.networking.k8s.io",
				Kind:  "Gateway",
				Name:  "does-not-exist",
			},
		},
		RuleSet: wafv1alpha1.WAFPolicyRuleSetRef{
			Name: "test-ruleset",
		},
		FailurePolicy: wafv1alpha1.FailurePolicyFail,
	}

	require.NoError(t, k8sClient.Create(ctx, policy))
	t.Cleanup(func() {
		_ = k8sClient.Delete(ctx, policy)
	})

	reconciler := newTestWAFPolicyReconciler()
	result, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      policy.Name,
			Namespace: policy.Namespace,
		},
	})

	// Should set not-accepted status (returns nil error) and requeue
	assert.NoError(t, err)
	assert.True(t, result.Requeue)

	var updated wafv1alpha1.WAFPolicy
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{
		Name:      policy.Name,
		Namespace: policy.Namespace,
	}, &updated))

	require.Len(t, updated.Status.Conditions, 1)
	assert.Equal(t, "Accepted", updated.Status.Conditions[0].Type)
	assert.Equal(t, "False", string(updated.Status.Conditions[0].Status))
	assert.Equal(t, "TargetNotFound", updated.Status.Conditions[0].Reason)
}

func TestWAFPolicyReconciler_ValidationRejectsInvalidTargetRef(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		group         string
		kind          string
		targetName    string
		expectedError string
	}{
		{
			name:          "wrong group",
			group:         "apps",
			kind:          "Gateway",
			targetName:    "test",
			expectedError: "targetRef.group must be gateway.networking.k8s.io",
		},
		{
			name:          "wrong kind",
			group:         "gateway.networking.k8s.io",
			kind:          "Service",
			targetName:    "test",
			expectedError: "targetRef.kind must be Gateway or HTTPRoute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &wafv1alpha1.WAFPolicy{}
			policy.Name = "validation-test-" + tt.name
			policy.Namespace = "default"
			policy.Spec = wafv1alpha1.WAFPolicySpec{
				TargetRef: gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName{
					LocalPolicyTargetReference: gwapiv1.LocalPolicyTargetReference{
						Group: gwapiv1.Group(tt.group),
						Kind:  gwapiv1.Kind(tt.kind),
						Name:  gwapiv1.ObjectName(tt.targetName),
					},
				},
				RuleSet: wafv1alpha1.WAFPolicyRuleSetRef{
					Name: "test-ruleset",
				},
				FailurePolicy: wafv1alpha1.FailurePolicyFail,
			}

			err := k8sClient.Create(ctx, policy)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}
