# kubectl-coraza

`kubectl-coraza` is a **kubectl plugin**: install the binary as `kubectl-coraza` on your `PATH` so Kubernetes exposes it as **`kubectl coraza`** ([plugin mechanism](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)).

It provides client-side helpers for the Coraza Kubernetes operator. Today the main command generates **RuleSet**, **ConfigMap**, and optional **Secret** manifests from OWASP CoreRuleSet files on disk. The operator validates and compiles rules after you apply resources; this tool does not compile Coraza rules.

## Install

Build or copy the binary so the name is exactly `kubectl-coraza` and it is on `PATH`. Then:

```bash
kubectl coraza version
```

## Generate CoreRuleSet manifests

```bash
kubectl coraza generate coreruleset \
  --rules-dir /path/to/rules \
  --version 4.24.1 \
  [--namespace my-ns] \
  [--ruleset-name default-ruleset] \
  [flags...]
```

Reads `*.conf` and optional `*.data` from **one directory** (non-recursive). Writes a **multi-document YAML stream** to stdout; progress and warnings go to stderr.

### Common flags

| Flag | Meaning |
|------|---------|
| `--rules-dir` | Directory with `*.conf` / `*.data` (required) |
| `--version` | CRS version, e.g. `4.24.1` or `v4.24.1` (required) |
| `-n`, `--namespace` | If set, `metadata.namespace` on every object |
| `--ruleset-name` | RuleSet `metadata.name` (default `default-ruleset`) |
| `--data-secret-name` | Secret name for `*.data` (default `coreruleset-data`) |
| `--ignore-rules` | Comma-separated rule IDs to drop |
| `--ignore-pmFromFile` | Strip `SecRule` lines using `@pmFromFile` |
| `--include-test-rule` | Append X-CRS-Test block to bundled `base-rules` |
| `--name-prefix` / `--name-suffix` | Prefix/suffix for ConfigMap names derived from `.conf` filenames |
| `--dry-run=client` | Same YAML; stderr notes dry-run (no cluster access) |
| `--skip-size-check` | Allow very large payloads (discouraged; etcd may still reject) |

---

## Library: `corerulesetgen`

Generation logic lives in [`../../pkg/corerulesetgen`](../../pkg/corerulesetgen). Use it if you need the pipeline without the kubectl wrapper.

It emits a bundled `base-rules` ConfigMap, one ConfigMap per non-empty `.conf`, an optional `coraza/data` Secret for `.data` files, and a `RuleSet` (`waf.k8s.coraza.io/v1alpha1`). It does **not** validate Coraza syntax beyond formatting and size checks.

### Pipeline

1. **`ParseCRSVersion(version string)`** — normalize CRS version (e.g. `v4.24.1` → `CRSVersion`).
2. **`Scan(rulesDir string)`** — glob `*.conf` / `*.data`, detect `@pmFromFile` in `.conf`.
3. **`Build(opts, scan, ver)`** — build a **`ManifestBundle`** (YAML + per-file results for logging).
4. **`WriteManifests(w, bundle)`** — multi-doc output with `---` separators.

**`Generate(stdout, opts)`** applies defaults, parses version, validates the rules directory, runs the same steps, and writes human-oriented progress to **`opts.Stderr`** (what the plugin uses).

### Example

```go
ver, err := corerulesetgen.ParseCRSVersion("4.24.1")
if err != nil { /* ... */ }

scan, err := corerulesetgen.Scan("/path/to/rules")
if err != nil { /* ... */ }

bundle, err := corerulesetgen.Build(corerulesetgen.Options{
    RulesDir:    "/path/to/rules",
    Version:     "4.24.1",
    RuleSetName: "my-ruleset",
}, scan, ver)
if err != nil { /* ... */ }

if err := corerulesetgen.WriteManifests(os.Stdout, bundle); err != nil { /* ... */ }
```

Defaults for **`Options`** match **`Generate`**; see [`../../pkg/corerulesetgen/generate.go`](../../pkg/corerulesetgen/generate.go).

### Tests

```bash
go test ./pkg/corerulesetgen/...
```

Golden fixtures: [`../../pkg/corerulesetgen/testdata`](../../pkg/corerulesetgen/testdata).
