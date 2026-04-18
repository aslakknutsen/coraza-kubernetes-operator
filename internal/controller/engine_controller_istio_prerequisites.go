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
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultIstioPrerequisitesReconcileInterval is the default interval at which
// the operator re-applies the Istio ServiceEntry and DestinationRule.
// Set to 0 to disable periodic re-apply (startup-only).
const DefaultIstioPrerequisitesReconcileInterval = 5 * time.Minute

// -----------------------------------------------------------------------------
// Istio Prerequisites
// -----------------------------------------------------------------------------

// IstioPrerequisites creates the Istio ServiceEntry and DestinationRule
// resources required for the RuleSet cache server to be reachable from
// Envoy sidecars within the mesh. Resources are applied at startup and,
// when reconcileInterval > 0, periodically re-applied via server-side
// apply so they self-heal after accidental deletion or drift.
type IstioPrerequisites struct {
	client            client.Client
	reader            client.Reader
	operatorName      string
	namespace         string
	istioRevision     string
	reconcileInterval time.Duration
}

// NewIstioPrerequisites returns a new IstioPrerequisites runnable.
// The reader should be a direct API reader (not cached) to avoid
// setting up a cluster-wide Deployment informer for a one-shot lookup.
//
// When reconcileInterval is positive, Start will re-apply the resources
// on that cadence. Zero or negative disables periodic re-apply (startup only).
func NewIstioPrerequisites(c client.Client, reader client.Reader, operatorName, namespace, istioRevision string, reconcileInterval time.Duration) *IstioPrerequisites {
	return &IstioPrerequisites{
		client:            c,
		reader:            reader,
		operatorName:      operatorName,
		namespace:         namespace,
		istioRevision:     istioRevision,
		reconcileInterval: reconcileInterval,
	}
}

// Start applies the Istio ServiceEntry and DestinationRule for the
// RuleSet cache server and, when reconcileInterval > 0, re-applies
// them periodically until ctx is cancelled. It satisfies the
// manager.Runnable interface and never returns a non-nil error from
// apply failures (the manager stays running).
func (p *IstioPrerequisites) Start(ctx context.Context) error {
	log := ctrl.Log.WithName("istio-prerequisites")

	if err := p.apply(ctx, log); err != nil {
		log.Error(err, "Failed to apply Istio prerequisites (controllers will continue without them)")
	}

	if p.reconcileInterval <= 0 {
		log.Info("Periodic Istio prerequisites reconciliation is disabled")
		return nil
	}

	log.Info("Starting periodic Istio prerequisites reconciliation", "interval", p.reconcileInterval)
	ticker := time.NewTicker(p.reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping periodic Istio prerequisites reconciliation")
			return nil
		case <-ticker.C:
			if err := p.apply(ctx, log); err != nil {
				log.Error(err, "Failed to re-apply Istio prerequisites")
			}
		}
	}
}

func (p *IstioPrerequisites) apply(ctx context.Context, log logr.Logger) error {
	var deploy appsv1.Deployment
	if err := p.reader.Get(ctx, types.NamespacedName{Name: p.operatorName, Namespace: p.namespace}, &deploy); err != nil {
		return fmt.Errorf("looking up owner Deployment %s/%s: %w", p.namespace, p.operatorName, err)
	}
	ownerRef := metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       deploy.Name,
		UID:        deploy.UID,
	}

	serviceFQDN := fmt.Sprintf("%s.%s.svc.cluster.local", p.operatorName, p.namespace)
	resourceName := p.operatorName + "-ruleset-cache"

	labels := map[string]string{
		"app.kubernetes.io/name":     p.operatorName,
		"app.kubernetes.io/instance": p.operatorName,
	}
	if p.istioRevision != "" {
		labels["istio.io/rev"] = p.istioRevision
	}

	se := p.buildServiceEntry(resourceName, serviceFQDN, labels, ownerRef)
	log.Info("Applying ServiceEntry", "name", resourceName, "namespace", p.namespace)
	if err := serverSideApply(ctx, p.client, se); err != nil {
		return fmt.Errorf("applying Istio ServiceEntry: %w", err)
	}

	dr := p.buildDestinationRule(resourceName, serviceFQDN, labels, ownerRef)
	log.Info("Applying DestinationRule", "name", resourceName, "namespace", p.namespace)
	if err := serverSideApply(ctx, p.client, dr); err != nil {
		return fmt.Errorf("applying Istio DestinationRule: %w", err)
	}

	return nil
}

func (p *IstioPrerequisites) newIstioObject(kind, name string, labels map[string]string, ownerRef metav1.OwnerReference, spec map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "networking.istio.io", Version: "v1", Kind: kind,
	})
	obj.SetName(name)
	obj.SetNamespace(p.namespace)
	obj.SetLabels(labels)
	obj.SetOwnerReferences([]metav1.OwnerReference{ownerRef})
	if obj.Object == nil {
		obj.Object = map[string]any{}
	}
	obj.Object["spec"] = spec
	return obj
}

func (p *IstioPrerequisites) buildServiceEntry(name, serviceFQDN string, labels map[string]string, ownerRef metav1.OwnerReference) *unstructured.Unstructured {
	return p.newIstioObject("ServiceEntry", name, labels, ownerRef, map[string]any{
		"hosts": []any{serviceFQDN},
		"ports": []any{
			map[string]any{
				"number":   int64(80),
				"name":     "http-ruleset-cache-server",
				"protocol": "HTTP",
			},
		},
		"location":   "MESH_INTERNAL",
		"resolution": "DNS",
		"endpoints": []any{
			map[string]any{
				"address": serviceFQDN,
			},
		},
	})
}

func (p *IstioPrerequisites) buildDestinationRule(name, serviceFQDN string, labels map[string]string, ownerRef metav1.OwnerReference) *unstructured.Unstructured {
	return p.newIstioObject("DestinationRule", name, labels, ownerRef, map[string]any{
		"host": serviceFQDN,
		"trafficPolicy": map[string]any{
			"tls": map[string]any{
				"mode": "DISABLE",
			},
		},
	})
}
