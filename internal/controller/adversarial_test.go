package controller

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"

	wafv1alpha1 "github.com/networking-incubator/coraza-kubernetes-operator/api/v1alpha1"
)

// ---------------------------------------------------------------------------
// truncateEventNote
// ---------------------------------------------------------------------------

func TestTruncateEventNote_Adversarial(t *testing.T) {
	t.Run("exactly max bytes unchanged", func(t *testing.T) {
		s := strings.Repeat("a", maxEventNoteBytes)
		assert.Equal(t, s, truncateEventNote(s))
	})

	t.Run("one byte over max truncates", func(t *testing.T) {
		s := strings.Repeat("a", maxEventNoteBytes+1)
		got := truncateEventNote(s)
		assert.Len(t, got, maxEventNoteBytes)
		assert.True(t, strings.HasSuffix(got, "..."))
	})

	t.Run("empty string", func(t *testing.T) {
		assert.Equal(t, "", truncateEventNote(""))
	})

	t.Run("unicode at boundary may split rune", func(t *testing.T) {
		// 3-byte rune (€) repeated to overshoot; truncation is byte-based
		s := strings.Repeat("€", maxEventNoteBytes)
		got := truncateEventNote(s)
		assert.LessOrEqual(t, len(got), maxEventNoteBytes)
	})
}

// ---------------------------------------------------------------------------
// sanitizeErrorMessage
// ---------------------------------------------------------------------------

func TestSanitizeErrorMessage_Adversarial(t *testing.T) {
	t.Run("non-matching error returned as-is", func(t *testing.T) {
		err := errors.New("something else went wrong")
		assert.Equal(t, err, sanitizeErrorMessage(err))
	})

	t.Run("matching open path uses basename only", func(t *testing.T) {
		err := errors.New("open /var/data/../../../etc/shadow: no such file or directory")
		got := sanitizeErrorMessage(err)
		assert.Contains(t, got.Error(), "shadow")
		assert.NotContains(t, got.Error(), "/var/data")
	})

	t.Run("wrapped nil inner does not match path regex", func(t *testing.T) {
		err := fmt.Errorf("context: %w", errors.New("unrelated"))
		got := sanitizeErrorMessage(err)
		assert.Equal(t, err.Error(), got.Error())
	})
}

// ---------------------------------------------------------------------------
// shouldSkipMissingFileError
// ---------------------------------------------------------------------------

func TestShouldSkipMissingFileError_Adversarial(t *testing.T) {
	matchErr := errors.New("open /fs/data.txt: no such file or directory")

	t.Run("nil secretData", func(t *testing.T) {
		assert.False(t, shouldSkipMissingFileError(matchErr, nil))
	})

	t.Run("empty secretData", func(t *testing.T) {
		assert.False(t, shouldSkipMissingFileError(matchErr, map[string][]byte{}))
	})

	t.Run("matching filename", func(t *testing.T) {
		assert.True(t, shouldSkipMissingFileError(matchErr, map[string][]byte{"data.txt": {1}}))
	})

	t.Run("non-matching filename", func(t *testing.T) {
		assert.False(t, shouldSkipMissingFileError(matchErr, map[string][]byte{"other.txt": {1}}))
	})

	t.Run("non-matching error format", func(t *testing.T) {
		assert.False(t, shouldSkipMissingFileError(errors.New("timeout"), map[string][]byte{"x": {}}))
	})
}

// ---------------------------------------------------------------------------
// buildCacheReadyMessage
// ---------------------------------------------------------------------------

func TestBuildCacheReadyMessage_Adversarial(t *testing.T) {
	t.Run("empty namespace and name", func(t *testing.T) {
		msg := buildCacheReadyMessage("", "", "")
		assert.Contains(t, msg, "Successfully cached rules for /")
	})

	t.Run("no unsupported msg omits override", func(t *testing.T) {
		msg := buildCacheReadyMessage("ns", "rs", "")
		assert.NotContains(t, msg, "[annotation override]")
	})

	t.Run("long unsupportedMsg appended", func(t *testing.T) {
		long := strings.Repeat("x", 5000)
		msg := buildCacheReadyMessage("ns", "rs", long)
		assert.Contains(t, msg, "[annotation override]")
		assert.Contains(t, msg, long)
	})
}

// ---------------------------------------------------------------------------
// serverSideApply validation (no real client, just pre-patch checks)
// ---------------------------------------------------------------------------

func TestServerSideApply_Adversarial(t *testing.T) {
	t.Run("empty GVK returns error", func(t *testing.T) {
		obj := &unstructured.Unstructured{}
		obj.SetName("foo")
		err := serverSideApply(nil, nil, obj)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "GroupVersionKind")
	})

	t.Run("empty name returns error", func(t *testing.T) {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "g", Version: "v", Kind: "K"})
		err := serverSideApply(nil, nil, obj)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name")
	})
}

// ---------------------------------------------------------------------------
// annotationChangedPredicate
// ---------------------------------------------------------------------------

func TestAnnotationChangedPredicate_Adversarial(t *testing.T) {
	p := annotationChangedPredicate("test-key")

	t.Run("create returns false", func(t *testing.T) {
		assert.False(t, p.Create(event.CreateEvent{}))
	})
	t.Run("delete returns false", func(t *testing.T) {
		assert.False(t, p.Delete(event.DeleteEvent{}))
	})
	t.Run("generic returns false", func(t *testing.T) {
		assert.False(t, p.Generic(event.GenericEvent{}))
	})
	t.Run("update with nil objects returns false", func(t *testing.T) {
		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: nil, ObjectNew: nil}))
	})
}

// ---------------------------------------------------------------------------
// hasIstioWasmDriver
// ---------------------------------------------------------------------------

func TestHasIstioWasmDriver_Adversarial(t *testing.T) {
	t.Run("nil Driver", func(t *testing.T) {
		e := &wafv1alpha1.Engine{Spec: wafv1alpha1.EngineSpec{Driver: nil}}
		assert.False(t, hasIstioWasmDriver(e))
	})
	t.Run("nil Istio", func(t *testing.T) {
		e := &wafv1alpha1.Engine{Spec: wafv1alpha1.EngineSpec{Driver: &wafv1alpha1.DriverConfig{}}}
		assert.False(t, hasIstioWasmDriver(e))
	})
	t.Run("nil Wasm", func(t *testing.T) {
		e := &wafv1alpha1.Engine{Spec: wafv1alpha1.EngineSpec{Driver: &wafv1alpha1.DriverConfig{
			Istio: &wafv1alpha1.IstioDriverConfig{},
		}}}
		assert.False(t, hasIstioWasmDriver(e))
	})
	t.Run("all set", func(t *testing.T) {
		e := &wafv1alpha1.Engine{Spec: wafv1alpha1.EngineSpec{Driver: &wafv1alpha1.DriverConfig{
			Istio: &wafv1alpha1.IstioDriverConfig{Wasm: &wafv1alpha1.IstioWasmConfig{}},
		}}}
		assert.True(t, hasIstioWasmDriver(e))
	})
}

// ---------------------------------------------------------------------------
// buildWasmPlugin
// ---------------------------------------------------------------------------

func TestBuildWasmPlugin_Adversarial(t *testing.T) {
	baseEngine := func() wafv1alpha1.Engine {
		return wafv1alpha1.Engine{
			ObjectMeta: metav1.ObjectMeta{Name: "test-engine", Namespace: "ns"},
			Spec: wafv1alpha1.EngineSpec{
				RuleSet: wafv1alpha1.RuleSetReference{Name: "my-rs"},
				Driver: &wafv1alpha1.DriverConfig{
					Istio: &wafv1alpha1.IstioDriverConfig{
						Wasm: &wafv1alpha1.IstioWasmConfig{
							Image: "oci://example.com/wasm:v1",
							WorkloadSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "gw"},
							},
						},
					},
				},
			},
		}
	}

	t.Run("nil FailurePolicy defaults to fail", func(t *testing.T) {
		e := baseEngine()
		r := &EngineReconciler{}
		wp := r.buildWasmPlugin(&e)
		spec := wp.Object["spec"].(map[string]any)
		pc := spec["pluginConfig"].(map[string]any)
		assert.Equal(t, "fail", pc["failure_policy"])
	})

	t.Run("explicit allow FailurePolicy", func(t *testing.T) {
		e := baseEngine()
		fp := wafv1alpha1.FailurePolicyAllow
		e.Spec.FailurePolicy = &fp
		r := &EngineReconciler{}
		wp := r.buildWasmPlugin(&e)
		spec := wp.Object["spec"].(map[string]any)
		pc := spec["pluginConfig"].(map[string]any)
		assert.Equal(t, "allow", pc["failure_policy"])
	})

	t.Run("nil MatchLabels becomes empty map", func(t *testing.T) {
		e := baseEngine()
		e.Spec.Driver.Istio.Wasm.WorkloadSelector = &metav1.LabelSelector{}
		r := &EngineReconciler{}
		wp := r.buildWasmPlugin(&e)
		spec := wp.Object["spec"].(map[string]any)
		sel := spec["selector"].(map[string]any)
		ml := sel["matchLabels"].(map[string]string)
		assert.Empty(t, ml)
	})

	t.Run("empty istioRevision omits label", func(t *testing.T) {
		e := baseEngine()
		r := &EngineReconciler{istioRevision: ""}
		wp := r.buildWasmPlugin(&e)
		labels := wp.GetLabels()
		_, hasRev := labels["istio.io/rev"]
		assert.False(t, hasRev)
	})

	t.Run("non-empty istioRevision sets label", func(t *testing.T) {
		e := baseEngine()
		r := &EngineReconciler{istioRevision: "canary"}
		wp := r.buildWasmPlugin(&e)
		assert.Equal(t, "canary", wp.GetLabels()["istio.io/rev"])
	})

	t.Run("PollIntervalSeconds included when set", func(t *testing.T) {
		e := baseEngine()
		poll := int32(30)
		e.Spec.Driver.Istio.Wasm.RuleSetCacheServer = &wafv1alpha1.RuleSetCacheServerConfig{
			PollIntervalSeconds: &poll,
		}
		r := &EngineReconciler{}
		wp := r.buildWasmPlugin(&e)
		spec := wp.Object["spec"].(map[string]any)
		pc := spec["pluginConfig"].(map[string]any)
		assert.Equal(t, int32(30), pc["rule_reload_interval_seconds"])
	})

	// BUG DEMONSTRATION: nil WorkloadSelector panics in buildWasmPlugin.
	// The CRD validates workloadSelector is required when mode=gateway, but
	// the Go code does not defensively check.
	t.Run("nil WorkloadSelector panics", func(t *testing.T) {
		e := baseEngine()
		e.Spec.Driver.Istio.Wasm.WorkloadSelector = nil
		r := &EngineReconciler{}
		assert.Panics(t, func() {
			r.buildWasmPlugin(&e)
		})
	})
}

// ---------------------------------------------------------------------------
// getDataFilesystem
// ---------------------------------------------------------------------------

func TestGetDataFilesystem_Adversarial(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, getDataFilesystem(nil))
	})

	t.Run("empty map returns non-nil fs", func(t *testing.T) {
		fs := getDataFilesystem(map[string][]byte{})
		assert.NotNil(t, fs)
	})

	t.Run("map with empty value", func(t *testing.T) {
		fs := getDataFilesystem(map[string][]byte{"empty.dat": {}})
		require.NotNil(t, fs)
		f, err := fs.Open("empty.dat")
		require.NoError(t, err)
		defer f.Close()
		info, err := f.Stat()
		require.NoError(t, err)
		assert.Equal(t, int64(0), info.Size())
	})
}

// ---------------------------------------------------------------------------
// Error types
// ---------------------------------------------------------------------------

func TestSecretErrors_Adversarial(t *testing.T) {
	t.Run("secretNotFoundError includes name", func(t *testing.T) {
		e := &secretNotFoundError{name: "my-secret"}
		assert.Contains(t, e.Error(), "my-secret")
	})

	t.Run("secretTypeMismatchError mentions expected type", func(t *testing.T) {
		e := &secretTypeMismatchError{name: "my-secret"}
		assert.Contains(t, e.Error(), wafv1alpha1.RuleDataSecretType)
	})

	// The secretTypeMismatchError does NOT include the secret name in its
	// Error() output, making error messages less actionable.
	t.Run("secretTypeMismatchError omits secret name (known gap)", func(t *testing.T) {
		e := &secretTypeMismatchError{name: "my-secret"}
		assert.NotContains(t, e.Error(), "my-secret")
	})
}

// ---------------------------------------------------------------------------
// engineMatchesLabels — additional adversarial cases beyond existing tests
// ---------------------------------------------------------------------------

func TestEngineMatchesLabels_Adversarial(t *testing.T) {
	t.Run("invalid matchExpressions operator returns false", func(t *testing.T) {
		e := &wafv1alpha1.Engine{
			Spec: wafv1alpha1.EngineSpec{Driver: &wafv1alpha1.DriverConfig{
				Istio: &wafv1alpha1.IstioDriverConfig{
					Wasm: &wafv1alpha1.IstioWasmConfig{
						WorkloadSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      "app",
								Operator: metav1.LabelSelectorOperator("BadOp"),
								Values:   []string{"x"},
							}},
						},
					},
				},
			}},
		}
		assert.False(t, engineMatchesLabels(e, map[string]string{"app": "x"}))
	})

	t.Run("empty pod labels with restrictive matchLabels", func(t *testing.T) {
		e := &wafv1alpha1.Engine{
			Spec: wafv1alpha1.EngineSpec{Driver: &wafv1alpha1.DriverConfig{
				Istio: &wafv1alpha1.IstioDriverConfig{
					Wasm: &wafv1alpha1.IstioWasmConfig{
						WorkloadSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "gw"},
						},
					},
				},
			}},
		}
		assert.False(t, engineMatchesLabels(e, map[string]string{}))
	})
}
