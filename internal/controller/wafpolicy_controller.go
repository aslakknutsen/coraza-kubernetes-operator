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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	wafv1alpha1 "github.com/networking-incubator/coraza-kubernetes-operator/api/v1alpha1"
)

// -----------------------------------------------------------------------------
// WAFPolicy Controller - RBAC
// -----------------------------------------------------------------------------

// +kubebuilder:rbac:groups=waf.k8s.coraza.io,resources=wafpolicies,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=waf.k8s.coraza.io,resources=wafpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=waf.k8s.coraza.io,resources=engines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch

// -----------------------------------------------------------------------------
// WAFPolicy Controller - Constants
// -----------------------------------------------------------------------------

const (
	EngineNamePrefix = "wafpolicy-"

	wafPolicyControllerName = "waf.k8s.coraza.io/wafpolicy-controller"

	wafPolicyFinalizer = "waf.k8s.coraza.io/wafpolicy-finalizer"
)

// -----------------------------------------------------------------------------
// WAFPolicy Controller - Config
// -----------------------------------------------------------------------------

// WAFPolicyTranslatorConfig holds operator-level defaults used when
// translating a WAFPolicy into an Engine resource. These are implementation
// details that don't belong in the user-facing WAFPolicy spec.
type WAFPolicyTranslatorConfig struct {
	DefaultWasmImage        string
	DefaultPollInterval     int32
	EnvoyClusterName        string
}

// -----------------------------------------------------------------------------
// WAFPolicy Controller
// -----------------------------------------------------------------------------

// WAFPolicyReconciler translates WAFPolicy resources into Engine resources.
type WAFPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Config   WAFPolicyTranslatorConfig
}

// SetupWithManager sets up the controller with the Manager.
func (r *WAFPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gateway := &unstructured.Unstructured{}
	gateway.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "Gateway",
	})

	httproute := &unstructured.Unstructured{}
	httproute.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "HTTPRoute",
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&wafv1alpha1.WAFPolicy{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&wafv1alpha1.Engine{}).
		Watches(gateway, handler.EnqueueRequestsFromMapFunc(r.findPoliciesForGateway)).
		Watches(httproute, handler.EnqueueRequestsFromMapFunc(r.findPoliciesForHTTPRoute)).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[ctrl.Request](
				1*time.Second,
				1*time.Minute,
			),
		}).
		Named("wafpolicy").
		Complete(r)
}

// -----------------------------------------------------------------------------
// WAFPolicy Controller - Reconciler
// -----------------------------------------------------------------------------

func (r *WAFPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	logDebug(log, req, "WAFPolicy", "Starting reconciliation")
	var policy wafv1alpha1.WAFPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			logDebug(log, req, "WAFPolicy", "Resource not found")
			return ctrl.Result{}, nil
		}
		logError(log, req, "WAFPolicy", err, "Failed to get")
		return ctrl.Result{}, err
	}

	if !policy.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, req, &policy)
	}

	if !controllerutil.ContainsFinalizer(&policy, wafPolicyFinalizer) {
		controllerutil.AddFinalizer(&policy, wafPolicyFinalizer)
		if err := r.Update(ctx, &policy); err != nil {
			logError(log, req, "WAFPolicy", err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	targetRef := policy.Spec.TargetRef
	kind := string(targetRef.Kind)

	switch kind {
	case "Gateway":
		return r.reconcileForGateway(ctx, req, &policy)
	case "HTTPRoute":
		return r.reconcileForHTTPRoute(ctx, req, &policy)
	default:
		return ctrl.Result{}, r.setNotAccepted(ctx, req, &policy, "InvalidTargetRef",
			fmt.Sprintf("Unsupported targetRef kind: %s", kind))
	}
}

// -----------------------------------------------------------------------------
// WAFPolicy Controller - Gateway Targeting
// -----------------------------------------------------------------------------

func (r *WAFPolicyReconciler) reconcileForGateway(ctx context.Context, req ctrl.Request, policy *wafv1alpha1.WAFPolicy) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	gatewayName := string(policy.Spec.TargetRef.Name)

	gw := &unstructured.Unstructured{}
	gw.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "Gateway",
	})
	if err := r.Get(ctx, types.NamespacedName{Name: gatewayName, Namespace: policy.Namespace}, gw); err != nil {
		if apierrors.IsNotFound(err) {
			logInfo(log, req, "WAFPolicy", "Target Gateway not found", "gateway", gatewayName)
			return ctrl.Result{Requeue: true}, r.setNotAccepted(ctx, req, policy, "TargetNotFound",
				fmt.Sprintf("Gateway %q not found", gatewayName))
		}
		return ctrl.Result{}, err
	}

	workloadLabels := map[string]string{
		"gateway.networking.k8s.io/gateway-name": gatewayName,
	}

	return r.ensureEngine(ctx, req, policy, workloadLabels)
}

// -----------------------------------------------------------------------------
// WAFPolicy Controller - HTTPRoute Targeting
// -----------------------------------------------------------------------------

func (r *WAFPolicyReconciler) reconcileForHTTPRoute(ctx context.Context, req ctrl.Request, policy *wafv1alpha1.WAFPolicy) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	routeName := string(policy.Spec.TargetRef.Name)

	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "HTTPRoute",
	})
	if err := r.Get(ctx, types.NamespacedName{Name: routeName, Namespace: policy.Namespace}, route); err != nil {
		if apierrors.IsNotFound(err) {
			logInfo(log, req, "WAFPolicy", "Target HTTPRoute not found", "httproute", routeName)
			return ctrl.Result{Requeue: true}, r.setNotAccepted(ctx, req, policy, "TargetNotFound",
				fmt.Sprintf("HTTPRoute %q not found", routeName))
		}
		return ctrl.Result{}, err
	}

	parentRefs, found, err := unstructured.NestedSlice(route.Object, "spec", "parentRefs")
	if err != nil || !found || len(parentRefs) == 0 {
		return ctrl.Result{}, r.setNotAccepted(ctx, req, policy, "NoParentGateway",
			fmt.Sprintf("HTTPRoute %q has no parentRefs", routeName))
	}

	firstParent, ok := parentRefs[0].(map[string]interface{})
	if !ok {
		return ctrl.Result{}, r.setNotAccepted(ctx, req, policy, "InvalidParentRef",
			fmt.Sprintf("HTTPRoute %q has invalid parentRef", routeName))
	}
	gatewayName, _, _ := unstructured.NestedString(firstParent, "name")
	if gatewayName == "" {
		return ctrl.Result{}, r.setNotAccepted(ctx, req, policy, "InvalidParentRef",
			fmt.Sprintf("HTTPRoute %q parentRef has no gateway name", routeName))
	}

	logDebug(log, req, "WAFPolicy", "Resolved HTTPRoute parent gateway", "gateway", gatewayName)

	workloadLabels := map[string]string{
		"gateway.networking.k8s.io/gateway-name": gatewayName,
	}

	return r.ensureEngine(ctx, req, policy, workloadLabels)
}

// -----------------------------------------------------------------------------
// WAFPolicy Controller - Engine Creation
// -----------------------------------------------------------------------------

func (r *WAFPolicyReconciler) ensureEngine(ctx context.Context, req ctrl.Request, policy *wafv1alpha1.WAFPolicy, workloadLabels map[string]string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	engine := &wafv1alpha1.Engine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      EngineNamePrefix + policy.Name,
			Namespace: policy.Namespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, engine, func() error {
		engine.Spec = wafv1alpha1.EngineSpec{
			RuleSet: corev1.ObjectReference{
				APIVersion: "waf.k8s.coraza.io/v1alpha1",
				Kind:       "RuleSet",
				Name:       policy.Spec.RuleSet.Name,
			},
			FailurePolicy: policy.Spec.FailurePolicy,
			Driver: wafv1alpha1.DriverConfig{
				Istio: &wafv1alpha1.IstioDriverConfig{
					Wasm: &wafv1alpha1.IstioWasmConfig{
						Image: r.Config.DefaultWasmImage,
						Mode:  wafv1alpha1.IstioIntegrationModeGateway,
						WorkloadSelector: &metav1.LabelSelector{
							MatchLabels: workloadLabels,
						},
						RuleSetCacheServer: &wafv1alpha1.RuleSetCacheServerConfig{
							PollIntervalSeconds: r.Config.DefaultPollInterval,
						},
					},
				},
			},
		}
		return controllerutil.SetControllerReference(policy, engine, r.Scheme)
	})
	if err != nil {
		logError(log, req, "WAFPolicy", err, "Failed to ensure Engine")
		r.Recorder.Event(policy, "Warning", "EngineSyncFailed", fmt.Sprintf("Failed to create/update Engine: %v", err))

		patch := client.MergeFrom(policy.DeepCopy())
		setConditionFalse(&policy.Status.Conditions, policy.Generation, "Programmed", "EngineSyncFailed",
			fmt.Sprintf("Failed to create/update Engine: %v", err))
		if updateErr := r.Status().Patch(ctx, policy, patch); updateErr != nil {
			logError(log, req, "WAFPolicy", updateErr, "Failed to patch status")
		}
		return ctrl.Result{}, err
	}

	logInfo(log, req, "WAFPolicy", "Engine synced", "engine", engine.Name, "operation", result)

	patch := client.MergeFrom(policy.DeepCopy())
	apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
		Type:               "Accepted",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             "Accepted",
		Message:            "WAFPolicy is accepted",
	})
	apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
		Type:               "Programmed",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             "Programmed",
		Message:            fmt.Sprintf("Engine %q %s", engine.Name, result),
	})
	if err := r.Status().Patch(ctx, policy, patch); err != nil {
		logError(log, req, "WAFPolicy", err, "Failed to update status")
		return ctrl.Result{}, err
	}

	r.Recorder.Event(policy, "Normal", "Programmed", fmt.Sprintf("Engine %s/%s %s", engine.Namespace, engine.Name, result))
	return ctrl.Result{}, nil
}

// -----------------------------------------------------------------------------
// WAFPolicy Controller - Deletion Handling
// -----------------------------------------------------------------------------

func (r *WAFPolicyReconciler) handleDeletion(ctx context.Context, req ctrl.Request, policy *wafv1alpha1.WAFPolicy) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if controllerutil.ContainsFinalizer(policy, wafPolicyFinalizer) {
		logInfo(log, req, "WAFPolicy", "Removing finalizer")
		controllerutil.RemoveFinalizer(policy, wafPolicyFinalizer)
		if err := r.Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// -----------------------------------------------------------------------------
// WAFPolicy Controller - Status Helpers
// -----------------------------------------------------------------------------

func (r *WAFPolicyReconciler) setNotAccepted(ctx context.Context, req ctrl.Request, policy *wafv1alpha1.WAFPolicy, reason, message string) error {
	log := logf.FromContext(ctx)

	r.Recorder.Event(policy, "Warning", reason, message)
	patch := client.MergeFrom(policy.DeepCopy())
	apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
		Type:               "Accepted",
		Status:             metav1.ConditionFalse,
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})
	apimeta.RemoveStatusCondition(&policy.Status.Conditions, "Programmed")
	if err := r.Status().Patch(ctx, policy, patch); err != nil {
		logError(log, req, "WAFPolicy", err, "Failed to patch not-accepted status")
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// WAFPolicy Controller - Watch Mappers
// -----------------------------------------------------------------------------

func (r *WAFPolicyReconciler) findPoliciesForGateway(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.findPoliciesForTarget(ctx, "Gateway", obj.GetName(), obj.GetNamespace())
}

func (r *WAFPolicyReconciler) findPoliciesForHTTPRoute(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.findPoliciesForTarget(ctx, "HTTPRoute", obj.GetName(), obj.GetNamespace())
}

func (r *WAFPolicyReconciler) findPoliciesForTarget(ctx context.Context, kind, name, namespace string) []reconcile.Request {
	var policies wafv1alpha1.WAFPolicyList
	if err := r.List(ctx, &policies, client.InNamespace(namespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, p := range policies.Items {
		if string(p.Spec.TargetRef.Kind) == kind && string(p.Spec.TargetRef.Name) == name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      p.Name,
					Namespace: p.Namespace,
				},
			})
		}
	}
	return requests
}
