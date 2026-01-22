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
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// -----------------------------------------------------------------------------
// Logging Utilities
// -----------------------------------------------------------------------------

// debugLevel is the go-logr level for debug/verbose logging
const debugLevel = 1

// logInfo logs an info-level message with consistent structured context.
func logInfo(log logr.Logger, req ctrl.Request, kind, msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"namespace", req.Namespace, "name", req.Name}, keysAndValues...)
	log.Info(fmt.Sprintf("%s: %s", kind, msg), args...)
}

// logDebug logs a debug-level message with consistent structured context.
func logDebug(log logr.Logger, req ctrl.Request, kind, msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"namespace", req.Namespace, "name", req.Name}, keysAndValues...)
	log.V(debugLevel).Info(fmt.Sprintf("%s: %s", kind, msg), args...)
}

// logError logs an error-level message with consistent structured context.
func logError(log logr.Logger, req ctrl.Request, kind string, err error, msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"namespace", req.Namespace, "name", req.Name}, keysAndValues...)
	log.Error(err, fmt.Sprintf("%s: %s", kind, msg), args...)
}

// -----------------------------------------------------------------------------
// Status Condition Utilities
// -----------------------------------------------------------------------------

// setConditionTrue is a helper function to set metav1.Conditions to True.
func setConditionTrue(conditions *[]metav1.Condition, generation int64, conditionType, reason, message string) {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// setConditionFalse is a helper function to set metav1.Conditions to False.
func setConditionFalse(conditions *[]metav1.Condition, generation int64, conditionType, reason, message string) {
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
}

// setStatusConditionDegraded is a helper to mark a resource as degraded.
func setStatusConditionDegraded(log logr.Logger, req ctrl.Request, kind string, conditions *[]metav1.Condition, generation int64, reason, message string) {
	logDebug(log, req, kind, fmt.Sprintf("Setting degraded status: %s", reason))
	setConditionFalse(conditions, generation, "Ready", reason, message)
	setConditionTrue(conditions, generation, "Degraded", reason, message)
	apimeta.RemoveStatusCondition(conditions, "Progressing")
}

// setStatusProgressing is a helper to mark a resource as actively progressing.
func setStatusProgressing(log logr.Logger, req ctrl.Request, kind string, conditions *[]metav1.Condition, generation int64, reason, message string) {
	logDebug(log, req, kind, fmt.Sprintf("Setting progressing status: %s", reason))
	setConditionFalse(conditions, generation, "Ready", reason, message)
	setConditionTrue(conditions, generation, "Progressing", reason, message)
}

// setStatusReady is a helper to mark a resource as ready, fully reconciled.
func setStatusReady(log logr.Logger, req ctrl.Request, kind string, conditions *[]metav1.Condition, generation int64, reason, message string) {
	logDebug(log, req, kind, fmt.Sprintf("Setting ready status: %s", reason))
	setConditionTrue(conditions, generation, "Ready", reason, message)
	apimeta.RemoveStatusCondition(conditions, "Degraded")
	apimeta.RemoveStatusCondition(conditions, "Progressing")
}

// -----------------------------------------------------------------------------
// Kubernetes Client Operation Utilities
// -----------------------------------------------------------------------------

// createOrUpdate creates or updates an unstructured Kubernetes object.
// If the object doesn't exist, it creates it. If it exists, it updates it.
//
// The desired object must have its GVK and name set.
func createOrUpdate(ctx context.Context, c client.Client, desired *unstructured.Unstructured) error {
	gvk := desired.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		return errors.New("desired object must have GroupVersionKind set")
	}

	namespace, name := desired.GetNamespace(), desired.GetName()
	if name == "" {
		return errors.New("desired object must have a name set")
	}
	if namespace == "" {
		namespace = corev1.NamespaceDefault
	}

	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(desired.GetObjectKind().GroupVersionKind())

	err := c.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      desired.GetName(),
	}, resource)

	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := c.Create(ctx, desired); err != nil {
				return fmt.Errorf("failed to create %s/%s in namespace %s: %w", gvk.Kind, name, namespace, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get %s/%s in namespace %s: %w", gvk.Kind, name, namespace, err)
	}

	desired.SetResourceVersion(resource.GetResourceVersion())

	if err := c.Update(ctx, desired); err != nil {
		return fmt.Errorf("failed to update %s/%s in namespace %s: %w", gvk.Kind, name, namespace, err)
	}

	return nil
}
