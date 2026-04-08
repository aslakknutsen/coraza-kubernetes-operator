/*
Copyright Coraza Kubernetes Operator contributors.

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
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	wafv1alpha1 "github.com/networking-incubator/coraza-kubernetes-operator/api/v1alpha1"
)

// ---------------------------------------------------------------------------
// captureSink — records log entries for assertions
// ---------------------------------------------------------------------------

type captureSink struct {
	entries []logEntry
}

type logEntry struct {
	Level         int
	Msg           string
	KeysAndValues []any
	Err           error
}

func (s *captureSink) Init(logr.RuntimeInfo)          {}
func (s *captureSink) Enabled(int) bool               { return true }
func (s *captureSink) WithValues(...any) logr.LogSink { return s }
func (s *captureSink) WithName(string) logr.LogSink   { return s }

func (s *captureSink) Info(level int, msg string, keysAndValues ...any) {
	s.entries = append(s.entries, logEntry{Level: level, Msg: msg, KeysAndValues: keysAndValues})
}

func (s *captureSink) Error(err error, msg string, keysAndValues ...any) {
	s.entries = append(s.entries, logEntry{Level: -1, Msg: msg, KeysAndValues: keysAndValues, Err: err})
}

func newCaptureLogger() (logr.Logger, *captureSink) {
	sink := &captureSink{}
	return logr.New(sink), sink
}

func kvMap(kvs []any) map[string]any {
	m := make(map[string]any, len(kvs)/2)
	for i := 0; i+1 < len(kvs); i += 2 {
		if k, ok := kvs[i].(string); ok {
			m[k] = kvs[i+1]
		}
	}
	return m
}

func testReq() ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "obj"}}
}

func TestBuildCacheReadyMessage(t *testing.T) {
	t.Run("without unsupported message", func(t *testing.T) {
		msg := buildCacheReadyMessage("ns", "my-rules", "")
		assert.Equal(t, "Successfully cached rules for ns/my-rules", msg)
	})

	t.Run("with unsupported message", func(t *testing.T) {
		msg := buildCacheReadyMessage("ns", "my-rules", "found unsupported rule 950150")
		assert.Contains(t, msg, "Successfully cached rules for ns/my-rules")
		assert.Contains(t, msg, "[annotation override]")
		assert.Contains(t, msg, "950150")
	})
}

func TestCollectRequests(t *testing.T) {
	engines := []wafv1alpha1.Engine{
		{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns2"}},
	}

	t.Run("empty slice returns nil", func(t *testing.T) {
		got := collectRequests([]wafv1alpha1.Engine{}, func(e *wafv1alpha1.Engine) bool {
			return true
		})
		assert.Nil(t, got)
	})

	t.Run("no matches returns nil", func(t *testing.T) {
		got := collectRequests(engines, func(e *wafv1alpha1.Engine) bool {
			return false
		})
		assert.Nil(t, got)
	})

	t.Run("all match", func(t *testing.T) {
		got := collectRequests(engines, func(e *wafv1alpha1.Engine) bool {
			return true
		})
		assert.Equal(t, []reconcile.Request{
			{NamespacedName: types.NamespacedName{Name: "a", Namespace: "ns1"}},
			{NamespacedName: types.NamespacedName{Name: "b", Namespace: "ns1"}},
			{NamespacedName: types.NamespacedName{Name: "c", Namespace: "ns2"}},
		}, got)
	})

	t.Run("partial match", func(t *testing.T) {
		got := collectRequests(engines, func(e *wafv1alpha1.Engine) bool {
			return e.Namespace == "ns1"
		})
		assert.Equal(t, []reconcile.Request{
			{NamespacedName: types.NamespacedName{Name: "a", Namespace: "ns1"}},
			{NamespacedName: types.NamespacedName{Name: "b", Namespace: "ns1"}},
		}, got)
	})
}

// ---------------------------------------------------------------------------
// extractStatusErrorFields
// ---------------------------------------------------------------------------

func TestExtractStatusErrorFields(t *testing.T) {
	t.Run("non-StatusError returns nil", func(t *testing.T) {
		fields := extractStatusErrorFields(fmt.Errorf("plain error"))
		assert.Nil(t, fields)
	})

	t.Run("StatusError returns code and reason", func(t *testing.T) {
		err := apierrors.NewNotFound(
			schema.GroupResource{Group: "waf.k8s.coraza.io", Resource: "engines"}, "my-engine")

		fields := extractStatusErrorFields(err)
		require.NotNil(t, fields)
		kv := kvMap(fields)
		assert.Equal(t, int32(http.StatusNotFound), kv["apiStatusCode"])
		assert.Equal(t, string(metav1.StatusReasonNotFound), kv["apiReason"])
	})

	t.Run("wrapped StatusError is detected", func(t *testing.T) {
		inner := apierrors.NewConflict(
			schema.GroupResource{Resource: "engines"}, "my-engine", fmt.Errorf("conflict"))
		wrapped := fmt.Errorf("fetching engine: %w", inner)

		fields := extractStatusErrorFields(wrapped)
		require.NotNil(t, fields)
		kv := kvMap(fields)
		assert.Equal(t, int32(http.StatusConflict), kv["apiStatusCode"])
	})

	t.Run("StatusError with RetryAfterSeconds includes field", func(t *testing.T) {
		err := &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    http.StatusTooManyRequests,
				Reason:  metav1.StatusReasonTooManyRequests,
				Details: &metav1.StatusDetails{RetryAfterSeconds: 30},
			},
		}

		fields := extractStatusErrorFields(err)
		kv := kvMap(fields)
		assert.Equal(t, int32(30), kv["retryAfterSeconds"])
		assert.Equal(t, int32(http.StatusTooManyRequests), kv["apiStatusCode"])
	})

	t.Run("StatusError without RetryAfterSeconds omits field", func(t *testing.T) {
		err := apierrors.NewInternalError(fmt.Errorf("internal"))

		fields := extractStatusErrorFields(err)
		kv := kvMap(fields)
		_, hasRetry := kv["retryAfterSeconds"]
		assert.False(t, hasRetry)
	})
}

// ---------------------------------------------------------------------------
// logAPIError
// ---------------------------------------------------------------------------

func TestLogAPIError(t *testing.T) {
	req := testReq()

	t.Run("nil obj omits resourceVersion", func(t *testing.T) {
		log, sink := newCaptureLogger()
		logAPIError(log, req, "Engine", fmt.Errorf("test"), "Failed to get", nil)

		require.Len(t, sink.entries, 1)
		kv := kvMap(sink.entries[0].KeysAndValues)
		_, hasRV := kv["resourceVersion"]
		assert.False(t, hasRV)
	})

	t.Run("obj with resourceVersion adds it", func(t *testing.T) {
		log, sink := newCaptureLogger()

		obj := &unstructured.Unstructured{}
		obj.SetResourceVersion("12345")

		err := apierrors.NewNotFound(schema.GroupResource{Resource: "engines"}, "eng")
		logAPIError(log, req, "Engine", err, "Failed to get", obj)

		require.Len(t, sink.entries, 1)
		kv := kvMap(sink.entries[0].KeysAndValues)
		assert.Equal(t, "12345", kv["resourceVersion"])
		assert.Equal(t, int32(http.StatusNotFound), kv["apiStatusCode"])
	})

	t.Run("non-nil obj with empty resourceVersion omits key", func(t *testing.T) {
		log, sink := newCaptureLogger()
		obj := &unstructured.Unstructured{}
		obj.SetName("x")

		err := apierrors.NewTimeoutError("slow", 0)
		logAPIError(log, req, "Engine", err, "Failed", obj)

		kv := kvMap(sink.entries[0].KeysAndValues)
		_, hasRV := kv["resourceVersion"]
		assert.False(t, hasRV)
	})

	t.Run("extra key-value pairs are included", func(t *testing.T) {
		log, sink := newCaptureLogger()
		logAPIError(log, req, "RuleSet", fmt.Errorf("err"), "oops", nil, "secretName", "my-secret")

		require.Len(t, sink.entries, 1)
		kv := kvMap(sink.entries[0].KeysAndValues)
		assert.Equal(t, "my-secret", kv["secretName"])
	})

	t.Run("odd extra args drops only trailing orphan, keeps valid pairs", func(t *testing.T) {
		log, sink := newCaptureLogger()
		logAPIError(log, req, "Engine", fmt.Errorf("err"), "Failed", nil,
			"validKey", "validVal", "orphanKey")

		var hasDebugWarning bool
		for _, e := range sink.entries {
			if e.Level == debugLevel && strings.Contains(e.Msg, "odd number of extra") {
				hasDebugWarning = true
			}
		}
		assert.True(t, hasDebugWarning, "expected debug warning about odd extra args")

		var errorEntry *logEntry
		for i := range sink.entries {
			if sink.entries[i].Level == -1 {
				errorEntry = &sink.entries[i]
			}
		}
		require.NotNil(t, errorEntry, "error should still be logged")
		kv := kvMap(errorEntry.KeysAndValues)
		assert.Equal(t, "ns", kv["namespace"])
		assert.Equal(t, "obj", kv["name"])
		assert.Equal(t, "validVal", kv["validKey"], "valid pair before the orphan must be preserved")
		_, hasOrphan := kv["orphanKey"]
		assert.False(t, hasOrphan, "orphan key should not appear")
	})

	t.Run("single orphan extra arg drops it entirely", func(t *testing.T) {
		log, sink := newCaptureLogger()
		logAPIError(log, req, "Engine", fmt.Errorf("err"), "Failed", nil, "orphanKey")

		var errorEntry *logEntry
		for i := range sink.entries {
			if sink.entries[i].Level == -1 {
				errorEntry = &sink.entries[i]
			}
		}
		require.NotNil(t, errorEntry)
		kv := kvMap(errorEntry.KeysAndValues)
		assert.Equal(t, "ns", kv["namespace"])
		_, hasOrphan := kv["orphanKey"]
		assert.False(t, hasOrphan)
	})
}
