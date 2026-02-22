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
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	wafv1alpha1 "github.com/networking-incubator/coraza-kubernetes-operator/api/v1alpha1"
)

// -----------------------------------------------------------------------------
// Engine Controller - Gateway Status - RBAC
// -----------------------------------------------------------------------------

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;patch

// -----------------------------------------------------------------------------
// Engine Controller - Gateway Status - Constants
// -----------------------------------------------------------------------------

const (
	// gatewayNameLabel is the well-known label Istio sets on pods backing a
	// Gateway API Gateway resource.
	gatewayNameLabel = "gateway.networking.k8s.io/gateway-name"

	// gatewayStatusCleanupFinalizer is added to Engines that have set a
	// condition on a Gateway, ensuring the condition is removed on deletion.
	gatewayStatusCleanupFinalizer = "waf.k8s.coraza.io/gateway-status-cleanup"

	// gatewayConditionTypePrefix is the domain-prefixed condition type set
	// on Gateways to indicate Engine attachment status.
	gatewayConditionTypePrefix = "waf.k8s.coraza.io/EngineReady"
)

var gatewayGVK = schema.GroupVersionKind{
	Group:   "gateway.networking.k8s.io",
	Version: "v1",
	Kind:    "Gateway",
}

// -----------------------------------------------------------------------------
// Engine Controller - Gateway Status - Resolution
// -----------------------------------------------------------------------------

// resolveTargetGateway extracts the target Gateway name from the Engine's
// workloadSelector matchLabels. Returns empty string if no gateway can be
// resolved (which is not an error — the Engine may target non-Gateway
// workloads).
func resolveTargetGateway(engine *wafv1alpha1.Engine) (name, namespace string) {
	if engine.Spec.Driver.Istio == nil || engine.Spec.Driver.Istio.Wasm == nil {
		return "", ""
	}
	ws := engine.Spec.Driver.Istio.Wasm.WorkloadSelector
	if ws == nil || ws.MatchLabels == nil {
		return "", ""
	}
	gwName, ok := ws.MatchLabels[gatewayNameLabel]
	if !ok || gwName == "" {
		return "", ""
	}
	return gwName, engine.Namespace
}

// gatewayConditionType returns the per-engine condition type for a Gateway.
func gatewayConditionType(engineName string) string {
	return fmt.Sprintf("%s-%s", gatewayConditionTypePrefix, engineName)
}

// -----------------------------------------------------------------------------
// Engine Controller - Gateway Status - Set / Remove Conditions
// -----------------------------------------------------------------------------

// setGatewayEngineCondition applies the Engine's condition to the Gateway's
// status subresource using server-side apply. The condition type is unique
// per engine to support multiple Engines targeting the same Gateway.
//
// Each Engine uses its own field manager (scoped by engine name) so that
// multiple Engines can independently own their respective conditions on the
// same Gateway without SSA removing each other's entries.
func (r *EngineReconciler) setGatewayEngineCondition(
	ctx context.Context,
	gatewayName, gatewayNamespace string,
	engine *wafv1alpha1.Engine,
	conditionStatus metav1.ConditionStatus,
	reason, message string,
) error {
	gw := &unstructured.Unstructured{}
	gw.SetGroupVersionKind(gatewayGVK)
	if err := r.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: gatewayNamespace}, gw); err != nil {
		return err
	}

	condType := gatewayConditionType(engine.Name)
	now := metav1.Now()

	condition := map[string]interface{}{
		"type":               condType,
		"status":             string(conditionStatus),
		"reason":             reason,
		"message":            message,
		"lastTransitionTime": now.UTC().Format("2006-01-02T15:04:05Z"),
		"observedGeneration": gw.GetGeneration(),
	}

	patch := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "Gateway",
			"metadata": map[string]interface{}{
				"name":      gatewayName,
				"namespace": gatewayNamespace,
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{condition},
			},
		},
	}
	patch.SetGroupVersionKind(gatewayGVK)

	fm := fmt.Sprintf("%s/%s", fieldManager, engine.Name)
	return r.Status().Patch(ctx, patch, client.Apply, client.FieldOwner(fm), client.ForceOwnership)
}

// removeGatewayEngineCondition removes the Engine's condition from the
// Gateway's status using server-side apply with an empty conditions list.
// SSA removes only the entries owned by this engine's field manager,
// leaving conditions set by other engines or controllers untouched.
//
// If the Gateway is not found (already deleted), this returns nil.
func (r *EngineReconciler) removeGatewayEngineCondition(
	ctx context.Context,
	gatewayName, gatewayNamespace, engineName string,
) error {
	patch := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "Gateway",
			"metadata": map[string]interface{}{
				"name":      gatewayName,
				"namespace": gatewayNamespace,
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{},
			},
		},
	}
	patch.SetGroupVersionKind(gatewayGVK)

	fm := fmt.Sprintf("%s/%s", fieldManager, engineName)
	if err := r.Status().Patch(ctx, patch, client.Apply, client.FieldOwner(fm), client.ForceOwnership); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to remove Gateway condition via SSA: %w", err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Engine Controller - Gateway Status - Reconcile Integration
// -----------------------------------------------------------------------------

// reconcileGatewayStatus resolves the target Gateway and sets the appropriate
// condition based on the Engine's current state. Returns the resolved gateway
// info for recording in Engine status and a non-nil error if the condition
// could not be applied (callers should requeue on transient errors).
func (r *EngineReconciler) reconcileGatewayStatus(
	ctx context.Context,
	log logr.Logger,
	req ctrl.Request,
	engine *wafv1alpha1.Engine,
	conditionStatus metav1.ConditionStatus,
	reason, message string,
) ([]wafv1alpha1.TargetGatewayStatus, error) {
	gwName, gwNamespace := resolveTargetGateway(engine)
	if gwName == "" {
		return nil, nil
	}

	logDebug(log, req, "Engine", "Resolved target Gateway", "gatewayName", gwName, "gatewayNamespace", gwNamespace)

	if err := r.ensureFinalizer(ctx, engine); err != nil {
		logError(log, req, "Engine", err, "Failed to add finalizer")
		return nil, fmt.Errorf("failed to ensure gateway cleanup finalizer: %w", err)
	}

	attached := true
	var retErr error
	if err := r.setGatewayEngineCondition(ctx, gwName, gwNamespace, engine, conditionStatus, reason, message); err != nil {
		attached = false
		if apierrors.IsNotFound(err) {
			logInfo(log, req, "Engine", "Target Gateway not found", "gatewayName", gwName)
			r.Recorder.Event(engine, "Warning", "GatewayNotFound", fmt.Sprintf("Target Gateway %s/%s not found", gwNamespace, gwName))
		} else {
			logError(log, req, "Engine", err, "Failed to set Gateway condition", "gatewayName", gwName)
			retErr = fmt.Errorf("failed to set Gateway condition: %w", err)
		}
	} else {
		logInfo(log, req, "Engine", "Set condition on Gateway", "gatewayName", gwName, "conditionStatus", conditionStatus)
	}

	return []wafv1alpha1.TargetGatewayStatus{{
		Name:      gwName,
		Namespace: gwNamespace,
		Attached:  attached,
	}}, retErr
}

// -----------------------------------------------------------------------------
// Engine Controller - Gateway Status - Finalizer
// -----------------------------------------------------------------------------

// ensureFinalizer adds the gateway-status-cleanup finalizer if not already
// present. It re-fetches the engine to avoid conflicts with status patches.
func (r *EngineReconciler) ensureFinalizer(ctx context.Context, engine *wafv1alpha1.Engine) error {
	if controllerutil.ContainsFinalizer(engine, gatewayStatusCleanupFinalizer) {
		return nil
	}
	patch := client.MergeFrom(engine.DeepCopy())
	controllerutil.AddFinalizer(engine, gatewayStatusCleanupFinalizer)
	return r.Patch(ctx, engine, patch)
}

// handleDeletion processes the finalizer during Engine deletion: removes the
// Gateway condition and then removes the finalizer. Returns true if the
// Engine is being deleted and the caller should return early.
func (r *EngineReconciler) handleDeletion(
	ctx context.Context,
	log logr.Logger,
	req ctrl.Request,
	engine *wafv1alpha1.Engine,
) (bool, error) {
	if engine.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if !controllerutil.ContainsFinalizer(engine, gatewayStatusCleanupFinalizer) {
		return true, nil
	}

	logInfo(log, req, "Engine", "Processing deletion finalizer")

	// Use status as the authoritative record of which gateways actually have
	// conditions. Fall back to spec-based resolution for the case where the
	// finalizer was added but reconciliation never wrote status.
	var gateways []wafv1alpha1.TargetGatewayStatus
	if len(engine.Status.TargetGateways) > 0 {
		gateways = engine.Status.TargetGateways
	} else {
		gwName, gwNamespace := resolveTargetGateway(engine)
		if gwName != "" {
			gateways = []wafv1alpha1.TargetGatewayStatus{{Name: gwName, Namespace: gwNamespace}}
		}
	}

	for _, gw := range gateways {
		if err := r.removeGatewayEngineCondition(ctx, gw.Name, gw.Namespace, engine.Name); err != nil {
			logError(log, req, "Engine", err, "Failed to remove Gateway condition during cleanup")
			return true, err
		}
		logInfo(log, req, "Engine", "Removed condition from Gateway", "gatewayName", gw.Name)
	}

	patch := client.MergeFrom(engine.DeepCopy())
	controllerutil.RemoveFinalizer(engine, gatewayStatusCleanupFinalizer)
	if err := r.Patch(ctx, engine, patch); err != nil {
		logError(log, req, "Engine", err, "Failed to remove finalizer")
		return true, err
	}

	logInfo(log, req, "Engine", "Finalizer removed, deletion can proceed")
	return true, nil
}
