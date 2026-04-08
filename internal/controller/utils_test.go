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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	wafv1alpha1 "github.com/networking-incubator/coraza-kubernetes-operator/api/v1alpha1"
	"github.com/networking-incubator/coraza-kubernetes-operator/internal/rulesets/cache"
)

func TestCacheEntryPayloadSize(t *testing.T) {
	tests := []struct {
		name      string
		rules     string
		dataFiles map[string][]byte
		expected  int
	}{
		{
			name:      "empty rules, nil data",
			rules:     "",
			dataFiles: nil,
			expected:  0,
		},
		{
			name:      "rules only",
			rules:     "SecRule REQUEST_URI",
			dataFiles: nil,
			expected:  len("SecRule REQUEST_URI"),
		},
		{
			name:  "rules plus data files",
			rules: "abc",
			dataFiles: map[string][]byte{
				"file.data": []byte("content"),
			},
			expected: len("abc") + len("file.data") + len("content"),
		},
		{
			name:  "multiple data files",
			rules: "",
			dataFiles: map[string][]byte{
				"a.data": []byte("x"),
				"b.data": []byte("yy"),
			},
			expected: len("a.data") + 1 + len("b.data") + 2,
		},
		{
			name:      "empty data map",
			rules:     "r",
			dataFiles: map[string][]byte{},
			expected:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cacheEntryPayloadSize(tt.rules, tt.dataFiles)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestCacheEntryPayloadSize_matchesRuleSetCacheTotalSize(t *testing.T) {
	c := cache.NewRuleSetCache()
	rules := "SecRule"
	data := map[string][]byte{"rule1.data": []byte("payload")}
	want := cacheEntryPayloadSize(rules, data)
	c.Put("test/instance", rules, data)
	assert.Equal(t, want, c.TotalSize(), "payload accounting must match RuleSetCache.TotalSize for a single stored entry")
}

func TestOversizedAdmissionUsesStrictGreaterThanBudget(t *testing.T) {
	budget := 1000
	rules := strings.Repeat("a", 1000)
	assert.Equal(t, 1000, cacheEntryPayloadSize(rules, nil))
	assert.False(t, cacheEntryPayloadSize(rules, nil) > budget, "equal payload must be admitted (check uses > not >=)")
	assert.True(t, cacheEntryPayloadSize(rules+"x", nil) > budget)
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
