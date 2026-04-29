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

// Package metrics defines coraza_engine_* and coraza_ruleset_* Prometheus
// metrics for the operator's custom resources. Metrics are registered on
// controller-runtime's global registry.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	MetricEngineInfo         = "coraza_engine_info"
	MetricEngineCondition    = "coraza_engine_condition"
	MetricRuleSetCondition   = "coraza_ruleset_condition"
)

var conditionStatuses = []string{
	string(metav1.ConditionTrue),
	string(metav1.ConditionFalse),
	string(metav1.ConditionUnknown),
}

var trackedConditionTypes = []string{"Ready", "Degraded", "Progressing"}

var (
	engineInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricEngineInfo,
			Help: "Info metric for Engine custom resources. Value is always 1 for active engines.",
		},
		[]string{"namespace", "name", "failure_policy"},
	)

	engineCondition = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricEngineCondition,
			Help: "Condition status for Engine custom resources. For each (namespace, name, type), exactly one status is 1.",
		},
		[]string{"namespace", "name", "type", "status"},
	)

	ruleSetCondition = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: MetricRuleSetCondition,
			Help: "Condition status for RuleSet custom resources. For each (namespace, name, type), exactly one status is 1.",
		},
		[]string{"namespace", "name", "type", "status"},
	)
)

func init() {
	metrics.Registry.MustRegister(engineInfo, engineCondition, ruleSetCondition)
}

// RecordEngineInfo sets the coraza_engine_info gauge for the given Engine.
// failurePolicy is the literal spec value (may be empty if user omitted it).
func RecordEngineInfo(namespace, name, failurePolicy string) {
	engineInfo.WithLabelValues(namespace, name, failurePolicy).Set(1)
}

// RecordEngineConditions updates coraza_engine_condition gauges for all
// tracked condition types. For each type, the matching status gets value 1
// and the other two get 0. If a condition type is absent from the list,
// all three statuses are set to 0.
func RecordEngineConditions(namespace, name string, conditions []metav1.Condition) {
	recordConditions(engineCondition, namespace, name, conditions)
}

// RecordRuleSetConditions updates coraza_ruleset_condition gauges for all
// tracked condition types, following the same semantics as RecordEngineConditions.
func RecordRuleSetConditions(namespace, name string, conditions []metav1.Condition) {
	recordConditions(ruleSetCondition, namespace, name, conditions)
}

// DeleteEngineMetrics removes all metric series for the given Engine.
func DeleteEngineMetrics(namespace, name string) {
	engineInfo.DeletePartialMatch(prometheus.Labels{"namespace": namespace, "name": name})
	engineCondition.DeletePartialMatch(prometheus.Labels{"namespace": namespace, "name": name})
}

// DeleteRuleSetMetrics removes all metric series for the given RuleSet.
func DeleteRuleSetMetrics(namespace, name string) {
	ruleSetCondition.DeletePartialMatch(prometheus.Labels{"namespace": namespace, "name": name})
}

func recordConditions(vec *prometheus.GaugeVec, namespace, name string, conditions []metav1.Condition) {
	for _, ct := range trackedConditionTypes {
		activeStatus := findConditionStatus(conditions, ct)
		for _, s := range conditionStatuses {
			val := float64(0)
			if s == activeStatus {
				val = 1
			}
			vec.WithLabelValues(namespace, name, ct, s).Set(val)
		}
	}
}

func findConditionStatus(conditions []metav1.Condition, condType string) string {
	for i := range conditions {
		if conditions[i].Type == condType {
			return string(conditions[i].Status)
		}
	}
	return ""
}

// EngineInfoVec returns the underlying GaugeVec for testing purposes only.
func EngineInfoVec() *prometheus.GaugeVec { return engineInfo }

// EngineConditionVec returns the underlying GaugeVec for testing purposes only.
func EngineConditionVec() *prometheus.GaugeVec { return engineCondition }

// RuleSetConditionVec returns the underlying GaugeVec for testing purposes only.
func RuleSetConditionVec() *prometheus.GaugeVec { return ruleSetCondition }
