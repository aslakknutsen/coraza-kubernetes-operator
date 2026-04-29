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

package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func resetMetrics(t *testing.T) {
	t.Helper()
	engineInfo.Reset()
	engineCondition.Reset()
	ruleSetCondition.Reset()
}

func TestMetricsRegistered(t *testing.T) {
	problems, err := testutil.GatherAndLint(metrics.Registry,
		MetricEngineInfo,
		MetricEngineCondition,
		MetricRuleSetCondition,
	)
	require.NoError(t, err)
	assert.Empty(t, problems, "metric lint problems")
}

func TestRecordEngineInfo(t *testing.T) {
	resetMetrics(t)

	RecordEngineInfo("default", "my-engine", "fail")

	val := testutil.ToFloat64(engineInfo.WithLabelValues("default", "my-engine", "fail"))
	assert.Equal(t, float64(1), val)
}

func TestRecordEngineInfo_EmptyFailurePolicy(t *testing.T) {
	resetMetrics(t)

	RecordEngineInfo("default", "my-engine", "")

	val := testutil.ToFloat64(engineInfo.WithLabelValues("default", "my-engine", ""))
	assert.Equal(t, float64(1), val)
}

func TestRecordEngineInfo_FailurePolicyChangeDoesNotLeaveStaleSeries(t *testing.T) {
	resetMetrics(t)

	RecordEngineInfo("ns", "eng", "fail")
	assert.Equal(t, float64(1), testutil.ToFloat64(engineInfo.WithLabelValues("ns", "eng", "fail")))

	RecordEngineInfo("ns", "eng", "allow")

	assert.Equal(t, 1, testutil.CollectAndCount(engineInfo), "exactly one series per namespace/name")
	assert.Equal(t, float64(1), testutil.ToFloat64(engineInfo.WithLabelValues("ns", "eng", "allow")))
	assert.Equal(t, float64(0), testutil.ToFloat64(engineInfo.WithLabelValues("ns", "eng", "fail")))
}

func TestRecordEngineConditions_Ready(t *testing.T) {
	resetMetrics(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	RecordEngineConditions("ns", "eng", conditions)

	assert.Equal(t, float64(1), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Ready", "True")))
	assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Ready", "False")))
	assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Ready", "Unknown")))

	// Absent conditions should all be 0
	assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Degraded", "True")))
	assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Degraded", "False")))
	assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Degraded", "Unknown")))
}

func TestRecordEngineConditions_Degraded(t *testing.T) {
	resetMetrics(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionFalse},
		{Type: "Degraded", Status: metav1.ConditionTrue},
	}
	RecordEngineConditions("ns", "eng", conditions)

	assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Ready", "True")))
	assert.Equal(t, float64(1), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Ready", "False")))
	assert.Equal(t, float64(1), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Degraded", "True")))
	assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Degraded", "False")))
}

func TestRecordEngineConditions_AllAbsent(t *testing.T) {
	resetMetrics(t)

	RecordEngineConditions("ns", "eng", nil)

	for _, ct := range trackedConditionTypes {
		for _, s := range conditionStatuses {
			assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", ct, s)),
				"expected 0 for absent condition %s/%s", ct, s)
		}
	}
}

func TestRecordRuleSetConditions(t *testing.T) {
	resetMetrics(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	RecordRuleSetConditions("ns", "rs", conditions)

	assert.Equal(t, float64(1), testutil.ToFloat64(ruleSetCondition.WithLabelValues("ns", "rs", "Ready", "True")))
	assert.Equal(t, float64(0), testutil.ToFloat64(ruleSetCondition.WithLabelValues("ns", "rs", "Ready", "False")))
}

func TestRecordConditions_TransitionsCorrectly(t *testing.T) {
	resetMetrics(t)

	// Start Progressing
	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionFalse},
		{Type: "Progressing", Status: metav1.ConditionTrue},
	}
	RecordEngineConditions("ns", "eng", conditions)
	assert.Equal(t, float64(1), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Progressing", "True")))
	assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Ready", "True")))

	// Transition to Ready
	conditions = []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	RecordEngineConditions("ns", "eng", conditions)
	assert.Equal(t, float64(1), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Ready", "True")))
	assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Progressing", "True")))
	assert.Equal(t, float64(0), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Progressing", "False")))
}

func TestDeleteEngineMetrics(t *testing.T) {
	resetMetrics(t)

	RecordEngineInfo("ns", "eng", "fail")
	RecordEngineConditions("ns", "eng", []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	})

	// Verify metrics exist
	assert.Equal(t, float64(1), testutil.ToFloat64(engineInfo.WithLabelValues("ns", "eng", "fail")))
	assert.Equal(t, float64(1), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng", "Ready", "True")))

	DeleteEngineMetrics("ns", "eng")

	// After deletion, collecting should yield no series for this engine.
	count := testutil.CollectAndCount(engineInfo)
	assert.Equal(t, 0, count, "engine info should have no series after delete")

	count = testutil.CollectAndCount(engineCondition)
	assert.Equal(t, 0, count, "engine condition should have no series after delete")
}

func TestDeleteRuleSetMetrics(t *testing.T) {
	resetMetrics(t)

	RecordRuleSetConditions("ns", "rs", []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "Degraded", Status: metav1.ConditionFalse},
	})

	assert.Greater(t, testutil.CollectAndCount(ruleSetCondition), 0)

	DeleteRuleSetMetrics("ns", "rs")

	count := testutil.CollectAndCount(ruleSetCondition)
	assert.Equal(t, 0, count, "ruleset condition should have no series after delete")
}

func TestDeleteEngineMetrics_DoesNotAffectOtherEngines(t *testing.T) {
	resetMetrics(t)

	RecordEngineInfo("ns", "eng-a", "fail")
	RecordEngineInfo("ns", "eng-b", "allow")
	RecordEngineConditions("ns", "eng-a", []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	})
	RecordEngineConditions("ns", "eng-b", []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	})

	DeleteEngineMetrics("ns", "eng-a")

	// eng-b should still be present
	assert.Equal(t, float64(1), testutil.ToFloat64(engineInfo.WithLabelValues("ns", "eng-b", "allow")))
	assert.Equal(t, float64(1), testutil.ToFloat64(engineCondition.WithLabelValues("ns", "eng-b", "Ready", "True")))
}

func TestMetricMetadata(t *testing.T) {
	resetMetrics(t)

	t.Run("engine_info help and type", func(t *testing.T) {
		RecordEngineInfo("ns", "test", "fail")
		expected := `# HELP coraza_engine_info Info metric for Engine custom resources. Value is always 1 for active engines.
# TYPE coraza_engine_info gauge
`
		err := testutil.CollectAndCompare(engineInfo, strings.NewReader(expected+
			`coraza_engine_info{failure_policy="fail",name="test",namespace="ns"} 1
`))
		assert.NoError(t, err)
	})

	t.Run("engine_condition help and type", func(t *testing.T) {
		RecordEngineConditions("ns", "test", []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionTrue},
		})
		// Just verify the metric family exists with the right type
		count := testutil.CollectAndCount(engineCondition)
		assert.Greater(t, count, 0)
	})
}
