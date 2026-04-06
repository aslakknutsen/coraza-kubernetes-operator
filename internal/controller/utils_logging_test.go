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
)

// ---------------------------------------------------------------------------
// captureSink — records log entries for assertions
// ---------------------------------------------------------------------------

type captureSink struct {
	entries []logEntry
}

type logEntry struct {
	Level         int // -1 for Error
	Msg           string
	KeysAndValues []any
	Err           error
}

func (s *captureSink) Init(logr.RuntimeInfo)          {}
func (s *captureSink) Enabled(int) bool                { return true }
func (s *captureSink) WithValues(...any) logr.LogSink  { return s }
func (s *captureSink) WithName(string) logr.LogSink    { return s }

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

func (s *captureSink) infoEntries() []logEntry {
	var out []logEntry
	for _, e := range s.entries {
		if e.Level >= 0 {
			out = append(out, e)
		}
	}
	return out
}

func (s *captureSink) findEntry(substr string) *logEntry {
	for i := range s.entries {
		if strings.Contains(s.entries[i].Msg, substr) {
			return &s.entries[i]
		}
	}
	return nil
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

// ---------------------------------------------------------------------------
// Condition Transition Tests
// ---------------------------------------------------------------------------

func TestLogConditionTransitions(t *testing.T) {
	req := testReq()

	t.Run("no prior conditions -> setStatusProgressing logs new conditions", func(t *testing.T) {
		log, sink := newCaptureLogger()
		var conditions []metav1.Condition

		setStatusProgressing(log, req, "Engine", &conditions, 1, "Reconciling", "starting")

		infos := sink.infoEntries()
		require.NotEmpty(t, infos, "expected Info log for new conditions")

		readySet := sink.findEntry("Condition set")
		require.NotNil(t, readySet, "expected 'Condition set' for Ready=False")
	})

	t.Run("idempotent call does not log transitions", func(t *testing.T) {
		log, sink := newCaptureLogger()
		conditions := []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionTrue, Reason: "RulesCached"},
		}

		setStatusReady(log, req, "RuleSet", &conditions, 1, "RulesCached", "cached")

		for _, e := range sink.entries {
			assert.NotContains(t, e.Msg, "Condition changed")
			assert.NotContains(t, e.Msg, "Condition set")
		}
	})

	t.Run("message-only update same status and reason stays silent", func(t *testing.T) {
		log, sink := newCaptureLogger()
		conditions := []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Configured", Message: "old detail"},
		}

		setStatusReady(log, req, "Engine", &conditions, 1, "Configured", "new detail")

		assert.Empty(t, sink.infoEntries(), "Status+Reason unchanged: no Info noise")
	})

	t.Run("Progressing -> Ready logs Ready change and Progressing removal", func(t *testing.T) {
		log, sink := newCaptureLogger()
		conditions := []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionFalse, Reason: "Reconciling"},
			{Type: "Progressing", Status: metav1.ConditionTrue, Reason: "Reconciling"},
		}

		setStatusReady(log, req, "Engine", &conditions, 1, "Configured", "done")

		changedEntry := sink.findEntry("Condition changed")
		require.NotNil(t, changedEntry)
		kv := kvMap(changedEntry.KeysAndValues)
		assert.Equal(t, "Ready", kv["condition"])
		assert.Equal(t, "False", kv["fromStatus"])
		assert.Equal(t, "True", kv["toStatus"])

		removedEntry := sink.findEntry("Condition removed")
		require.NotNil(t, removedEntry)
		rkv := kvMap(removedEntry.KeysAndValues)
		assert.Equal(t, "Progressing", rkv["condition"])
	})

	t.Run("Ready -> Degraded logs transitions", func(t *testing.T) {
		log, sink := newCaptureLogger()
		conditions := []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Configured"},
		}

		setStatusConditionDegraded(log, req, "Engine", &conditions, 1, "Error", "something broke")

		infos := sink.infoEntries()
		require.GreaterOrEqual(t, len(infos), 2, "expect Ready change + Degraded set")

		readyChanged := sink.findEntry("Condition changed")
		require.NotNil(t, readyChanged)
		rkv := kvMap(readyChanged.KeysAndValues)
		assert.Equal(t, "Ready", rkv["condition"])
		assert.Equal(t, "True", rkv["fromStatus"])
		assert.Equal(t, "False", rkv["toStatus"])
	})

	t.Run("same status different reason still logs", func(t *testing.T) {
		log, sink := newCaptureLogger()
		conditions := []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionFalse, Reason: "OldReason"},
			{Type: "Degraded", Status: metav1.ConditionTrue, Reason: "OldReason"},
		}

		setStatusConditionDegraded(log, req, "RuleSet", &conditions, 1, "NewReason", "updated")

		entry := sink.findEntry("Condition changed")
		require.NotNil(t, entry, "reason-only change should log")
		kv := kvMap(entry.KeysAndValues)
		assert.Equal(t, "NewReason", kv["reason"])
	})

	t.Run("setStatusProgressing adds Progressing when absent", func(t *testing.T) {
		log, sink := newCaptureLogger()
		conditions := []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionFalse, Reason: "Error"},
			{Type: "Degraded", Status: metav1.ConditionTrue, Reason: "Error"},
		}

		setStatusProgressing(log, req, "RuleSet", &conditions, 2, "Retrying", "retry")

		progressingSet := sink.findEntry("Condition set")
		require.NotNil(t, progressingSet)
		kv := kvMap(progressingSet.KeysAndValues)
		assert.Equal(t, "Progressing", kv["condition"])

		// Ready reason changes from Error to Retrying
		readyChanged := sink.findEntry("Condition changed")
		require.NotNil(t, readyChanged)
		rkv := kvMap(readyChanged.KeysAndValues)
		assert.Equal(t, "Ready", rkv["condition"])
		assert.Equal(t, "Retrying", rkv["reason"])
	})
}

// ---------------------------------------------------------------------------
// extractStatusErrorFields Tests
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
// logAPIError Tests
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
}
