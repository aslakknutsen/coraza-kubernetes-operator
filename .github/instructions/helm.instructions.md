---
applyTo: "charts/**"
---

- CRDs in `crds/` are generated directly by `controller-gen` via `make manifests`. No manual sync is needed for CRDs.
- RBAC rules in the ClusterRole template must stay in sync with `config/rbac/role.yaml`. Run `make helm.sync` after any change to `kubebuilder:rbac` markers.
- All configurable values must appear in `values.yaml` with sensible defaults. Helpers that reference `.Values.*` keys not present in `values.yaml` should be avoided.
- PDB defaults must be safe for the default `replicaCount`. A PDB with `minAvailable >= replicaCount` blocks voluntary evictions.
