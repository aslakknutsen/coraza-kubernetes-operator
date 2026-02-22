# Design: Gateway Status for Engines (Issue #50)

## 1. Problem Statement

### What

When an Engine targets a Gateway (via `workloadSelector`), the Gateway
has no visible indication that a WAF Engine is attached to it. All status
information lives on the Engine resource. A user looking at a Gateway has
no way to know it's being affected by CKO.

### Why

Operators and platform teams inspect Gateway resources to understand their
state. Without status on the Gateway itself, discovering WAF attachment
requires knowing to look at Engine resources separately. The issue asks
for three things:

1. Indicate the Engine is present and active on the Gateway.
2. Surface any critical status from the Engine.
3. Provide a clear link back to the relevant Engine resource.

## 2. Proposed Approach

### 2.1 Gateway Status Condition (Primary Mechanism)

Set a custom condition on the Gateway's `.status.conditions` using Server
Side Apply (SSA). The Gateway API spec explicitly allows custom condition
types prefixed with a domain name. The CKO field manager
(`coraza-kubernetes-operator`) would own only its custom condition types,
avoiding conflicts with the Gateway controller (Istio).

Condition when an Engine is attached and ready:

```yaml
status:
  conditions:
    # ... existing Istio conditions (Accepted, Programmed) ...
    - type: waf.k8s.coraza.io/EngineReady
      status: "True"
      reason: EngineAttached
      message: "Engine default/coraza is ready"
      lastTransitionTime: "2026-02-22T12:00:00Z"
      observedGeneration: 3
```

Condition when the Engine is degraded:

```yaml
    - type: waf.k8s.coraza.io/EngineReady
      status: "False"
      reason: EngineDegraded
      message: "Engine default/coraza: ProvisioningFailed - Failed to create WasmPlugin"
```

Condition when the Engine is progressing:

```yaml
    - type: waf.k8s.coraza.io/EngineReady
      status: "Unknown"
      reason: EngineProgressing
      message: "Engine default/coraza is being reconciled"
```

**Multiple Engines targeting the same Gateway:** Use one condition per
Engine, with the type `waf.k8s.coraza.io/EngineReady-<engine-name>`.
This avoids collisions since conditions use `+listMapKey=type`.
Alternative: a single `waf.k8s.coraza.io/EngineReady` condition that
aggregates across all engines (simpler but less informative). The
per-engine approach is better because it provides a direct link back
to each Engine resource and avoids losing information.

### 2.2 Gateway Resolution

The Engine currently targets pods via `workloadSelector.matchLabels`. In
Istio Gateway mode, the convention is to use the label
`gateway.networking.k8s.io/gateway-name=<name>`, which Istio sets on
pods backing a Gateway API Gateway resource.

The Engine controller resolves the target Gateway by:

1. Checking that `spec.driver.istio.wasm.workloadSelector` is non-nil.
   The field is `*metav1.LabelSelector` and is `+optional`, so nil is
   a valid state. If nil, skip Gateway resolution.
2. Extracting the value of `gateway.networking.k8s.io/gateway-name`
   from `workloadSelector.matchLabels`.
3. Looking up the Gateway by that name in the Engine's namespace.

If `workloadSelector` is nil, or uses only `matchExpressions` without
a `matchLabels` entry for the gateway name label, the controller cannot
resolve a Gateway and skips status updates. This is a valid scenario
(the user might target non-Gateway workloads). No error is raised.

If the resolved Gateway doesn't exist, the Engine records a warning
event and sets its own status to Degraded with reason
`GatewayNotFound`. The Gateway condition is obviously not set (there's
nothing to set it on).

### 2.3 Engine Status Enhancement

Add a `targetGateways` field to `EngineStatus` to record the resolved
Gateway references:

```go
type EngineStatus struct {
    Conditions     []metav1.Condition     `json:"conditions,omitempty" ...`
    TargetGateways []TargetGatewayStatus  `json:"targetGateways,omitempty"`
}

type TargetGatewayStatus struct {
    // Name of the target Gateway.
    Name      string `json:"name"`
    // Namespace of the target Gateway.
    Namespace string `json:"namespace"`
    // Whether the Engine condition was successfully applied to this Gateway.
    Attached  bool   `json:"attached"`
}
```

This provides bidirectional discoverability: Gateway -> Engine (via
condition message) and Engine -> Gateway (via `targetGateways`).

### 2.4 Cleanup on Engine Deletion

When an Engine is deleted, its condition must be removed from the
Gateway's status. Two approaches:

**Option A: Finalizer.** Add a finalizer to the Engine. On deletion, the
controller removes the Gateway condition, then removes the finalizer.

**Option B: Periodic reconciliation from Gateway side.** A watch on
Gateways checks if referenced Engines still exist.

Recommend **Option A** (finalizer). It's the standard Kubernetes pattern
for cleanup of cross-resource state, and the Engine controller already
owns the lifecycle. The Gateway-side watch would require a separate
reconciler and is harder to reason about.

Finalizer name: `waf.k8s.coraza.io/gateway-status-cleanup`

**Edge case: Gateway deleted before Engine.** If the Gateway is already
gone when the finalizer runs, the cleanup call to remove the condition
will get a NotFound error. The finalizer logic must treat NotFound as
success (there's nothing left to clean up) and proceed to remove the
finalizer. Failing to handle this would block Engine deletion
indefinitely.

### 2.5 Controller Changes

**Engine controller (`engine_controller.go`):**

- After successful provisioning (or on degraded/progressing state), resolve
  the target Gateway and set the condition.
- Add a finalizer on Engine creation. On deletion, remove the condition
  from the Gateway, then remove the finalizer.
- Add a watch on Gateways to re-trigger Engine reconciliation when a
  Gateway is created/updated/deleted (so the Engine can re-apply its
  condition if the Gateway was recreated). The watch uses an
  `handler.EnqueueRequestsFromMapFunc` that, given a Gateway event,
  lists all Engines in the same namespace and filters to those whose
  `workloadSelector.matchLabels` contain
  `gateway.networking.k8s.io/gateway-name` matching the Gateway's name.
  This reverse lookup can use a client list with a field or label
  selector, or a simple list-and-filter (the number of Engines per
  namespace is expected to be small).

**New file: `engine_controller_gateway_status.go`:**

Encapsulate Gateway resolution and status update logic:

- `resolveTargetGateway(engine) -> (name, namespace, error)`
- `setGatewayEngineCondition(ctx, gateway, engine, conditionStatus, reason, message) error`
- `removeGatewayEngineCondition(ctx, gatewayName, gatewayNamespace, engineName) error`

All Gateway status access uses `unstructured.Unstructured` with SSA.

**Important:** The existing `serverSideApply()` helper in `utils.go` uses
`c.Patch()`, which patches the main resource. Gateway status lives on
the status subresource, so the new code must use `c.Status().Patch()`
instead. A new helper (e.g. `serverSideApplyStatus`) should be added
rather than reusing `serverSideApply()` directly.

**RBAC additions:**

```go
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;patch
```

### 2.6 SSA Field Ownership

The operator uses SSA with field manager `coraza-kubernetes-operator` for
WasmPlugin already. The same field manager applies the Engine condition
to the Gateway status. Because conditions are a list map keyed by `type`,
only the CKO-owned condition types are affected. The Gateway controller's
conditions (`Accepted`, `Programmed`) are owned by a different field
manager and are not touched.

Use `client.ForceOwnership` to avoid conflicts if the condition was
previously set by a different field manager (e.g., after operator
upgrade).

## 3. Alternatives Considered

### 3.1 Annotations on the Gateway

Set an annotation like `waf.k8s.coraza.io/engines: [{"name":"coraza","ready":true}]`
on the Gateway.

Rejected because:
- JSON in annotations is hard to read and maintain.
- Not visible in `kubectl get gateway` output.
- Not queryable with standard Kubernetes tooling.
- Doesn't match the conditions-based pattern used elsewhere in the Gateway
  API ecosystem.

### 3.2 Only update Engine status (no Gateway mutation)

Add `targetGateways` to Engine status but don't touch the Gateway at all.

Rejected because it doesn't satisfy the core requirement: a user looking
at the Gateway should see that a WAF is attached. This requires
something on the Gateway itself.

### 3.3 Full Policy Attachment (GEP-713)

Use `WAFPolicy` with `targetRef` and `PolicyAncestorStatus` as proposed
in issue #59.

Rejected for v0.3.0 because:
- #59 is scoped for v0.5.0 and requires a new CRD and controller.
- The maintainer explicitly pushed it out of the critical path.
- The custom condition approach is forward-compatible: when Policy
  Attachment lands, the condition type can be updated or replaced.
  No breaking change.

### 3.4 Single aggregated condition vs per-engine conditions

A single `waf.k8s.coraza.io/EngineReady` condition aggregating all
attached Engines.

Rejected because:
- Loses per-engine status detail.
- Requires coordination between Engine reconcilers (which one "owns"
  the aggregated condition?).
- The per-engine condition type (`waf.k8s.coraza.io/EngineReady-<name>`)
  is slightly more verbose but avoids these problems entirely.

## 4. Open Questions and Risks

1. **Cross-namespace Gateways.** The design assumes Engine and Gateway
   are in the same namespace (consistent with the current
   cross-namespace restriction on RuleSet references). If cross-namespace
   support is added later, Gateway resolution needs updating.

2. **Non-Istio drivers.** Gateway resolution is Istio-specific (relies
   on `gateway.networking.k8s.io/gateway-name` label convention). If
   other drivers are added, each driver needs its own resolution logic.
   The `resolveTargetGateway` function should be driver-dispatched, not
   generic.

3. **Condition type naming for multiple engines.** Using
   `waf.k8s.coraza.io/EngineReady-<engine-name>` embeds the engine name
   in the condition type. Kubernetes condition types are typically static
   strings. This is unconventional but not prohibited by the API.

   Recommendation: use the per-engine condition type as proposed. The
   alternatives (single aggregated condition, or encoding in message)
   all lose information or create ownership coordination problems
   between Engine reconcilers. The per-engine type is the lesser cost.
   If this turns out to cause friction with tooling that expects static
   types, it can be revisited when Policy Attachment (#59) lands and
   replaces this mechanism entirely.

4. **Gateway controller compatibility.** Tested assumption: Istio's
   gateway controller does not strip unknown conditions from Gateway
   status. This needs verification with the target Istio version.
   If Istio does strip them, the condition will be continuously
   re-applied and re-stripped (wasteful but not broken).

5. **SSA partial object.** When patching Gateway status via SSA, the
   patch object must include only the status fields we own. If the
   apply configuration includes fields we don't intend to own, we could
   inadvertently take ownership of Istio's conditions. The
   unstructured patch must be carefully constructed.

6. **Relation to #59 (Policy Attachment).** This design is a stepping
   stone. When WAFPolicy lands, the Gateway status mechanism may shift
   to `PolicyAncestorStatus` on the WAFPolicy resource, and the
   custom condition on the Gateway could be deprecated or replaced with
   the standard `gateway.networking.k8s.io/PolicyAffected` condition.

## 5. Suggested Implementation Order

1. **Add Gateway resolution logic.**
   New file `internal/controller/engine_controller_gateway_status.go`
   with `resolveTargetGateway`, `setGatewayEngineCondition`, and
   `removeGatewayEngineCondition` functions. Unit-testable in isolation.

2. **Add `TargetGatewayStatus` to Engine API types.**
   Update `api/v1alpha1/engine_types.go` with the new status struct.
   Run `make generate` and `make manifests` to regenerate deepcopy and
   CRD YAML.

3. **Wire Gateway status updates into Engine reconciler.**
   After successful provisioning / on degraded / on progressing, call
   the Gateway status functions. Add RBAC markers. Update `SetupWithManager`
   to watch Gateways.

4. **Add finalizer for cleanup.**
   On Engine creation, add finalizer. On deletion, remove Gateway
   condition, then remove finalizer.

5. **Add unit tests.**
   Test Gateway resolution (label present, label absent, Gateway not found).
   Test condition set/remove. Test finalizer lifecycle. Test multiple
   engines targeting same Gateway.

6. **Update integration tests.**
   After applying Engine, verify Gateway has the expected condition.
   After deleting Engine, verify condition is removed.
