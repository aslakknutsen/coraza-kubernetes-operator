/*
Copyright Coraza Kubernetes Operator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package controller

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	wafv1alpha1 "github.com/networking-incubator/coraza-kubernetes-operator/api/v1alpha1"
	"github.com/networking-incubator/coraza-kubernetes-operator/test/utils"
)

// -----------------------------------------------------------------------------
// Engine controller — helpers & nil chains
// -----------------------------------------------------------------------------

func TestAdversarialHasIstioWasmDriver(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		engine *wafv1alpha1.Engine
		want   bool
	}{
		{"nil driver", &wafv1alpha1.Engine{Spec: wafv1alpha1.EngineSpec{Driver: nil}}, false},
		{"nil istio", &wafv1alpha1.Engine{Spec: wafv1alpha1.EngineSpec{Driver: &wafv1alpha1.DriverConfig{}}}, false},
		{"nil wasm", &wafv1alpha1.Engine{Spec: wafv1alpha1.EngineSpec{Driver: &wafv1alpha1.DriverConfig{
			Istio: &wafv1alpha1.IstioDriverConfig{},
		}}}, false},
		{"full wasm", utils.NewTestEngine(utils.EngineOptions{}), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hasIstioWasmDriver(tc.engine))
		})
	}
}

func TestAdversarialSelectDriver_InvalidConfigurations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := logr.Discard()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "e"}}
	rec := &EngineReconciler{
		Client:   fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build(),
		Scheme:   scheme,
		Recorder: utils.NewTestRecorder(),
	}

	cases := []struct {
		name   string
		engine wafv1alpha1.Engine
	}{
		{"nil Driver", wafv1alpha1.Engine{ObjectMeta: metav1.ObjectMeta{Name: "e", Namespace: "ns"}, Spec: wafv1alpha1.EngineSpec{Driver: nil, RuleSet: wafv1alpha1.RuleSetReference{Name: "rs"}}}},
		{"Istio nil", wafv1alpha1.Engine{ObjectMeta: metav1.ObjectMeta{Name: "e", Namespace: "ns"}, Spec: wafv1alpha1.EngineSpec{
			Driver:  &wafv1alpha1.DriverConfig{},
			RuleSet: wafv1alpha1.RuleSetReference{Name: "rs"},
		}}},
		{"Wasm nil", wafv1alpha1.Engine{ObjectMeta: metav1.ObjectMeta{Name: "e", Namespace: "ns"}, Spec: wafv1alpha1.EngineSpec{
			Driver:  &wafv1alpha1.DriverConfig{Istio: &wafv1alpha1.IstioDriverConfig{}},
			RuleSet: wafv1alpha1.RuleSetReference{Name: "rs"},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rec.selectDriver(ctx, log, req, tc.engine)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid driver configuration")
		})
	}
}

func TestAdversarialBuildWasmPlugin_NilWasmPanics(t *testing.T) {
	t.Parallel()
	engine := utils.NewTestEngine(utils.EngineOptions{})
	engine.Spec.Driver.Istio.Wasm = nil
	r := &EngineReconciler{ruleSetCacheServerCluster: "c"}
	require.Panics(t, func() { r.buildWasmPlugin(engine) })
}

func TestAdversarialBuildWasmPlugin_NilWorkloadSelectorPanics(t *testing.T) {
	t.Parallel()
	engine := utils.NewTestEngine(utils.EngineOptions{})
	engine.Spec.Driver.Istio.Wasm.WorkloadSelector = nil
	r := &EngineReconciler{ruleSetCacheServerCluster: "c"}
	require.Panics(t, func() { r.buildWasmPlugin(engine) })
}

func TestAdversarialFindEnginesForGateway_NoWasmDriver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ns := testNamespace
	eng := utils.NewTestEngine(utils.EngineOptions{Name: "gw-map", Namespace: ns})
	eng.Spec.Driver = &wafv1alpha1.DriverConfig{} // invalid but listable
	rec := &EngineReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(eng).Build(),
	}
	reqs := rec.findEnginesForGateway(ctx, &unstructured.Unstructured{
		Object: map[string]any{"metadata": map[string]any{"name": "g", "namespace": ns}},
	})
	require.Empty(t, reqs)
}

// -----------------------------------------------------------------------------
// Engine controller — envtest: gateways, degradation, concurrency
// -----------------------------------------------------------------------------

func TestAdversarialMatchedGateways_EmptyPodList(t *testing.T) {
	ctx := context.Background()
	engine := utils.NewTestEngine(utils.EngineOptions{
		Name:      "adv-eng-empty",
		Namespace: testNamespace,
	})
	rec := &EngineReconciler{Client: k8sClient, ruleSetCacheServerCluster: "c"}
	gws, err := rec.matchedGateways(ctx, logr.Discard(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: engine.Namespace, Name: engine.Name}}, engine)
	require.NoError(t, err)
	assert.Empty(t, gws)
}

func TestAdversarialMatchedGateways_PodsWithoutGatewayLabel(t *testing.T) {
	ctx := context.Background()
	engine := utils.NewTestEngine(utils.EngineOptions{
		Name:      "adv-eng-nolabel",
		Namespace: testNamespace,
	})
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "adv-nop",
			Namespace: testNamespace,
			Labels:    map[string]string{"app": "gateway"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c", Image: "pause:latest"}},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, pod))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, pod) })

	rec := &EngineReconciler{Client: k8sClient, ruleSetCacheServerCluster: "c"}
	gws, err := rec.matchedGateways(ctx, logr.Discard(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: engine.Namespace, Name: engine.Name}}, engine)
	require.NoError(t, err)
	assert.Empty(t, gws)
}

func TestAdversarialMatchedGateways_NilWorkloadSelector(t *testing.T) {
	ctx := context.Background()
	engine := utils.NewTestEngine(utils.EngineOptions{Name: "adv-nil-ws", Namespace: testNamespace})
	engine.Spec.Driver.Istio.Wasm.WorkloadSelector = nil

	rec := &EngineReconciler{Client: k8sClient, ruleSetCacheServerCluster: "c"}
	gws, err := rec.matchedGateways(ctx, logr.Discard(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: engine.Namespace, Name: engine.Name}}, engine)
	require.NoError(t, err)
	assert.Nil(t, gws)
}

func TestAdversarialIsRuleSetDegraded_NilRuleSetStatus(t *testing.T) {
	ctx := context.Background()
	rs := utils.NewTestRuleSet(utils.RuleSetOptions{Name: "adv-rs-nilstat", Namespace: testNamespace})
	require.NoError(t, k8sClient.Create(ctx, rs))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, rs) })

	engine := utils.NewTestEngine(utils.EngineOptions{Name: "adv-e1", Namespace: testNamespace, RuleSetName: rs.Name})
	rec := &EngineReconciler{Client: k8sClient, Scheme: scheme, Recorder: utils.NewTestRecorder()}
	degraded, err := rec.isRuleSetDegraded(ctx, logr.Discard(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: engine.Namespace, Name: engine.Name}}, engine)
	require.NoError(t, err)
	assert.False(t, degraded)
}

func TestAdversarialIsRuleSetDegraded_StatusNoConditions(t *testing.T) {
	ctx := context.Background()
	rs := utils.NewTestRuleSet(utils.RuleSetOptions{Name: "adv-rs-nocond", Namespace: testNamespace})
	require.NoError(t, k8sClient.Create(ctx, rs))
	rs.Status = &wafv1alpha1.RuleSetStatus{Conditions: nil}
	require.NoError(t, k8sClient.Status().Update(ctx, rs))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, rs) })

	engine := utils.NewTestEngine(utils.EngineOptions{Name: "adv-e2", Namespace: testNamespace, RuleSetName: rs.Name})
	rec := &EngineReconciler{Client: k8sClient, Scheme: scheme, Recorder: utils.NewTestRecorder()}
	degraded, err := rec.isRuleSetDegraded(ctx, logr.Discard(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: engine.Namespace, Name: engine.Name}}, engine)
	require.NoError(t, err)
	assert.False(t, degraded)
}

func TestAdversarialEngineReconcile_ConcurrentSameRequest(t *testing.T) {
	ctx := context.Background()
	rs := utils.NewTestRuleSet(utils.RuleSetOptions{Name: "adv-rs-conc", Namespace: testNamespace})
	require.NoError(t, k8sClient.Create(ctx, rs))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, rs) })

	engine := utils.NewTestEngine(utils.EngineOptions{Name: "adv-conc-eng", Namespace: testNamespace, RuleSetName: rs.Name})
	require.NoError(t, k8sClient.Create(ctx, engine))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, engine) })

	rec := &EngineReconciler{
		Client:                    k8sClient,
		Scheme:                    scheme,
		Recorder:                  utils.NewTestRecorder(),
		ruleSetCacheServerCluster: "c",
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: engine.Namespace, Name: engine.Name}}

	var wg sync.WaitGroup
	var panicCount atomic.Int32
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if recover() != nil {
					panicCount.Add(1)
				}
			}()
			_, _ = rec.Reconcile(ctx, req)
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(0), panicCount.Load(), "concurrent reconcile should not panic")
}

// -----------------------------------------------------------------------------
// RuleSet controller — validation, secrets, predicates
// -----------------------------------------------------------------------------

func TestAdversarialAggregateRulesFromSources_EmptyRulesList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rs := &wafv1alpha1.RuleSet{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"},
		Spec:       wafv1alpha1.RuleSetSpec{Rules: []wafv1alpha1.RuleSourceReference{}},
	}
	rec := &RuleSetReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(rs).Build()}
	agg, aggErrs, done, err := rec.aggregateRulesFromSources(ctx, logr.Discard(), ctrl.Request{}, rs, nil)
	require.NoError(t, err)
	assert.False(t, done)
	assert.Empty(t, aggErrs)
	assert.Equal(t, "", agg)
}

func TestAdversarialValidateConfigMapRules_EmptyRulesKeyContent(t *testing.T) {
	t.Parallel()
	err := validateConfigMapRules("", "cm", nil)
	require.NoError(t, err)
}

func TestAdversarialValidateConfigMapRules_OnlyComments(t *testing.T) {
	t.Parallel()
	onlyComments := "# SecRuleEngine On\n# comment line\n"
	err := validateConfigMapRules(onlyComments, "cm", nil)
	if err != nil {
		t.Logf("comments-only rules rejected: %v", err)
	}
}

func TestAdversarialSanitizeErrorMessage_Formats(t *testing.T) {
	t.Parallel()
	t.Run("unix style matched", func(t *testing.T) {
		err := errors.New("open /etc/waf/foo.data: no such file or directory")
		out := sanitizeErrorMessage(err)
		assert.Contains(t, out.Error(), "foo.data")
		assert.Contains(t, out.Error(), "data does not exist")
	})
	t.Run("windows style not matched", func(t *testing.T) {
		err := errors.New(`open C:\data\foo.data: The system cannot find the file specified`)
		out := sanitizeErrorMessage(err)
		assert.Equal(t, err, out)
	})
	t.Run("no open prefix", func(t *testing.T) {
		err := errors.New("something else went wrong")
		assert.Equal(t, err, sanitizeErrorMessage(err))
	})
}

func TestAdversarialShouldSkipMissingFileError(t *testing.T) {
	t.Parallel()
	err := errors.New("open /x/y/z.file: no such file or directory")
	assert.False(t, shouldSkipMissingFileError(err, nil))
	assert.False(t, shouldSkipMissingFileError(err, map[string][]byte{}))
	assert.True(t, shouldSkipMissingFileError(err, map[string][]byte{"z.file": {1}}))
	assert.False(t, shouldSkipMissingFileError(errors.New("unrelated"), map[string][]byte{"z.file": {1}}))
}

func TestAdversarialAnnotationChangedPredicate_NilObjects(t *testing.T) {
	t.Parallel()
	p := annotationChangedPredicate("ann")
	assert.False(t, p.Update(event.UpdateEvent{ObjectOld: nil, ObjectNew: &corev1.Pod{}}))
	assert.False(t, p.Update(event.UpdateEvent{ObjectOld: &corev1.Pod{}, ObjectNew: nil}))
}

func TestAdversarialGetDataFilesystem_EmptyMap(t *testing.T) {
	t.Parallel()
	fs := getDataFilesystem(map[string][]byte{})
	require.NotNil(t, fs)
}

func TestAdversarialGetDataSecret_TypeSlightlyWrong(t *testing.T) {
	ctx := context.Background()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "adv-data-wrong-type", Namespace: testNamespace},
		Type:       corev1.SecretType("coraza/data "), // not equal to RuleDataSecretType
		Data:       map[string][]byte{"f": []byte("x")},
	}
	require.NoError(t, k8sClient.Create(ctx, secret))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, secret) })

	rec := &RuleSetReconciler{Client: k8sClient}
	_, err := rec.getDataSecret(ctx, secret.Name, testNamespace)
	require.Error(t, err)
	var tm *secretTypeMismatchError
	assert.True(t, errors.As(err, &tm))
}

func TestAdversarialCacheRules_NilCachePanics(t *testing.T) {
	t.Parallel()
	rec := &RuleSetReconciler{Cache: nil}
	rs := utils.NewTestRuleSet(utils.RuleSetOptions{})
	require.Panics(t, func() {
		_, _ = rec.cacheRules(context.Background(), logr.Discard(), ctrl.Request{}, rs, "rules", nil, "")
	})
}

func TestAdversarialLargeRulesString_Allocation(t *testing.T) {
	if testing.Short() {
		t.Skip("large string stress test")
	}
	t.Parallel()
	huge := strings.Repeat("#", 4*1024*1024)
	err := validateConfigMapRules(huge, "big-cm", nil)
	if err != nil {
		t.Logf("large comment-only rules: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Shared utils
// -----------------------------------------------------------------------------

func TestAdversarialTruncateEventNote_Boundary(t *testing.T) {
	t.Parallel()
	s1024 := strings.Repeat("a", 1024)
	assert.Len(t, truncateEventNote(s1024), 1024)
	s1025 := strings.Repeat("b", 1025)
	out := truncateEventNote(s1025)
	assert.Len(t, out, 1024)
	assert.True(t, strings.HasSuffix(out, "..."))
}

func TestAdversarialServerSideApply_EmptyGVK(t *testing.T) {
	t.Parallel()
	u := &unstructured.Unstructured{}
	u.SetName("x")
	u.SetNamespace("default")
	err := serverSideApply(context.Background(), k8sClient, u)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GroupVersionKind")
}

func TestAdversarialCollectRequests_EmptySlice(t *testing.T) {
	t.Parallel()
	got := collectRequests([]wafv1alpha1.Engine{}, func(e *wafv1alpha1.Engine) bool { return true })
	assert.Nil(t, got)
}

func TestAdversarialBuildCacheReadyMessage_VeryLongUnsupported(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("U", 50000)
	msg := buildCacheReadyMessage("ns", "n", long)
	assert.Contains(t, msg, "Successfully cached rules for ns/n")
	assert.Contains(t, msg, "[annotation override]")
	assert.Contains(t, msg, long)
}

func TestAdversarialLogHelpers_ZeroLogger(t *testing.T) {
	t.Parallel()
	var log logr.Logger
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "x", Namespace: "y"}}
	require.NotPanics(t, func() {
		logInfo(log, req, "K", "msg")
		logDebug(log, req, "K", "msg")
		logError(log, req, "K", errors.New("e"), "msg")
	})
}

func TestAdversarialSetCondition_NilSlicePointer(t *testing.T) {
	t.Parallel()
	var conds *[]metav1.Condition
	require.NotPanics(t, func() {
		setConditionTrue(conds, 1, "Ready", "R", "M")
		setConditionFalse(conds, 1, "Ready", "R", "M")
	})
	assert.Nil(t, conds)
}
