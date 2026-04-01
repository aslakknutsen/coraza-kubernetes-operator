/*
Copyright Coraza Kubernetes Operator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package v1alpha1

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// fullyPopulatedEngine returns an Engine with every field set (including nested
// pointers, slices, and maps) to exercise DeepCopy.
func fullyPopulatedEngine() *Engine {
	mode := IstioIntegrationModeGateway
	poll := int32(42)
	fp := FailurePolicyAllow
	now := metav1.NewTime(time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC))
	uid := types.UID("test-uid")
	gen := int64(7)
	delGrace := int64(30)

	return &Engine{
		TypeMeta: metav1.TypeMeta{
			APIVersion: GroupVersion.String(),
			Kind:       "Engine",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "engine-full",
			Namespace:                  "test-ns",
			UID:                        uid,
			ResourceVersion:            "rv-99",
			Generation:                 gen,
			CreationTimestamp:          now,
			DeletionTimestamp:          &now,
			DeletionGracePeriodSeconds: &delGrace,
			Labels: map[string]string{
				"app": "coraza", "tier": "waf",
			},
			Annotations: map[string]string{
				"anno": "val",
			},
			Finalizers: []string{"waf.k8s.coraza.io/finalizer"},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Name:       "owner",
					UID:        "owner-uid",
					Controller: func(b bool) *bool { return &b }(true),
				},
			},
		},
		Spec: EngineSpec{
			RuleSet: RuleSetReference{Name: "my-ruleset"},
			Driver: &DriverConfig{
				Istio: &IstioDriverConfig{
					Wasm: &IstioWasmConfig{
						Mode: &mode,
						WorkloadSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"gateway.networking.k8s.io/gateway-name": "gw",
							},
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "env",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"prod"},
								},
							},
						},
						Image: "oci://example.com/wasm:v1",
						RuleSetCacheServer: &RuleSetCacheServerConfig{
							PollIntervalSeconds: &poll,
						},
					},
				},
			},
			FailurePolicy: &fp,
		},
		Status: &EngineStatus{
			Conditions: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 3,
					LastTransitionTime: now,
					Reason:             "RulesApplied",
					Message:            "ok",
				},
				{
					Type:               "Progressing",
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 3,
					LastTransitionTime: now,
					Reason:             "Idle",
					Message:            "",
				},
			},
			Gateways: []GatewayReference{
				{Name: "gw-a"},
				{Name: "gw-b"},
			},
		},
	}
}

func fullyPopulatedRuleSet() *RuleSet {
	now := metav1.NewTime(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC))
	rd := "my-secret"
	return &RuleSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: GroupVersion.String(),
			Kind:       "RuleSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "ruleset-full",
			Namespace:       "test-ns",
			UID:             types.UID("rs-uid"),
			ResourceVersion: "rs-rv",
			Labels:          map[string]string{"k": "v"},
			Annotations:     map[string]string{"a": "b"},
		},
		Spec: RuleSetSpec{
			Rules: []RuleSourceReference{
				{Name: "cm-one"},
				{Name: "cm-two"},
			},
			RuleData: &rd,
		},
		Status: &RuleSetStatus{
			Conditions: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 1,
					LastTransitionTime: now,
					Reason:             "Compiled",
					Message:            "rules cached",
				},
			},
		},
	}
}

func mutateEngineDeeply(e *Engine) {
	e.Name = "mutated"
	e.Labels["app"] = "changed"
	e.Labels["new"] = "x"
	e.Annotations["anno"] = "mut"
	e.Finalizers = append(e.Finalizers, "extra")
	e.Spec.RuleSet.Name = "other-rs"
	fp := FailurePolicyFail
	e.Spec.FailurePolicy = &fp
	e.Spec.Driver.Istio.Wasm.Image = "oci://mutated"
	e.Spec.Driver.Istio.Wasm.WorkloadSelector.MatchLabels["gateway.networking.k8s.io/gateway-name"] = "other-gw"
	e.Spec.Driver.Istio.Wasm.WorkloadSelector.MatchExpressions[0].Values = []string{"dev"}
	e.Spec.Driver.Istio.Wasm.RuleSetCacheServer.PollIntervalSeconds = new(int32)
	*e.Spec.Driver.Istio.Wasm.RuleSetCacheServer.PollIntervalSeconds = 1
	e.Status.Conditions[0].Type = "Mutated"
	e.Status.Gateways[0].Name = "mut-gw"
}

func mutateRuleSetDeeply(r *RuleSet) {
	r.Spec.Rules[0].Name = "changed-cm"
	s := "other-secret"
	r.Spec.RuleData = &s
	r.Status.Conditions[0].Reason = "Other"
}

func TestTypesAdversarial_DeepCopyEngine_AllFieldsIndependent(t *testing.T) {
	t.Parallel()
	orig := fullyPopulatedEngine()
	snapshot := orig.DeepCopy()
	require.NotNil(t, snapshot)

	cpy := orig.DeepCopy()
	require.NotNil(t, cpy)
	mutateEngineDeeply(orig)

	if reflect.DeepEqual(cpy, orig) {
		t.Fatal("expected copy to differ from mutated original")
	}
	if !reflect.DeepEqual(cpy, snapshot) {
		t.Fatalf("DeepCopy not independent of mutations:\n%+v\nvs snapshot:\n%+v", cpy, snapshot)
	}
}

func TestTypesAdversarial_DeepCopyRuleSet_AllFieldsIndependent(t *testing.T) {
	t.Parallel()
	orig := fullyPopulatedRuleSet()
	snapshot := orig.DeepCopy()
	cpy := orig.DeepCopy()
	mutateRuleSetDeeply(orig)

	if !reflect.DeepEqual(cpy, snapshot) {
		t.Fatalf("copy diverged from initial snapshot")
	}
	if reflect.DeepEqual(cpy, orig) {
		t.Fatal("expected copy to differ from mutated original")
	}
}

func TestTypesAdversarial_ZeroValueEngineDeepCopyNoPanic(t *testing.T) {
	t.Parallel()
	var e Engine
	require.NotPanics(t, func() {
		out := e.DeepCopy()
		require.NotNil(t, out)
		assert.Nil(t, out.Status)
	})
}

func TestTypesAdversarial_ZeroValueRuleSetDeepCopyNoPanic(t *testing.T) {
	t.Parallel()
	var r RuleSet
	require.NotPanics(t, func() {
		out := r.DeepCopy()
		require.NotNil(t, out)
		assert.Nil(t, out.Status)
	})
}

func TestTypesAdversarial_Engine_NilStatusDeepCopyStaysNil(t *testing.T) {
	t.Parallel()
	e := &Engine{
		Spec: EngineSpec{RuleSet: RuleSetReference{Name: "rs"}},
	}
	assert.Nil(t, e.Status)
	cp := e.DeepCopy()
	require.NotNil(t, cp)
	assert.Nil(t, cp.Status)
}

func TestTypesAdversarial_RuleSetReference_EmptyName_AllowedAtRuntime(t *testing.T) {
	t.Parallel()
	// OpenAPI/CRD requires minLength 1; the Go struct does not validate.
	ref := RuleSetReference{Name: ""}
	e := &Engine{Spec: EngineSpec{RuleSet: ref}}
	cp := e.DeepCopy()
	require.NotNil(t, cp)
	assert.Equal(t, "", cp.Spec.RuleSet.Name)
}

func TestTypesAdversarial_FailurePolicyConstantsMatchCRDEnum(t *testing.T) {
	t.Parallel()
	// config/crd/bases/waf.k8s.coraza.io_engines.yaml enum: fail, allow
	crdEnums := map[string]struct{}{
		"fail":  {},
		"allow": {},
	}
	for _, fp := range []FailurePolicy{FailurePolicyFail, FailurePolicyAllow} {
		_, ok := crdEnums[string(fp)]
		assert.Truef(t, ok, "constant %q not in CRD enum", fp)
	}
	// Every CRD value has a Go constant
	assert.Equal(t, FailurePolicy("fail"), FailurePolicyFail)
	assert.Equal(t, FailurePolicy("allow"), FailurePolicyAllow)
}
