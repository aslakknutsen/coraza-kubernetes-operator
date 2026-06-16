# Sample: Coraza WAF with Istio Gateway

Deploys a Coraza WAF Engine in front of a simple echo service using the
Kubernetes Gateway API and Istio.

## What's included

| File | Description |
|------|-------------|
| `ruleset.yaml` | `RuleSource` (SecLang rules), `RuleData` (`@pmFromFile` data files), and a `RuleSet` referencing them via `spec.sources` and `spec.data` |
| `engine.yaml` | `Engine` CR that references the RuleSet and targets an Istio Gateway (`spec.driver.wasm.image` is optional) |
| `gateway.yaml` | Kubernetes Gateway API `Gateway` using the Istio gateway class |
| `httproute.yaml` | `HTTPRoute` that sends all traffic through the gateway to the echo service |
| `echo.yaml` | A simple echo Deployment and Service to act as the backend |
| `envoyfilter-waf-stats.yaml` | `EnvoyFilter` that extracts `waf_filter_*` Prometheus labels from coraza-proxy-wasm stat names (requires mesh `proxyStatsMatcher` for `waf_filter`) |

## Prerequisites

- Kubernetes cluster with Istio and Gateway API CRDs installed
- coraza-kubernetes-operator running in the cluster

For Prometheus metrics and Console graphs on Kind, use `MONITORING=1 make cluster.kind` (see [DEVELOPMENT.md](../DEVELOPMENT.md)).

## Deploy

All samples must be deployed to the same namespace.

```bash
kubectl apply -f config/samples/
```

When using Gateway API auto-deployment, apply `envoyfilter-waf-stats.yaml` after the gateway pod exists.

## Metrics

coraza-proxy-wasm exposes `waf_filter_*` counters on the Envoy stats port (`:15090/stats/prometheus`). Labels such as `rule_id`, `phase`, `engine`, and `namespace` require:

1. `metric_labels` in the WasmPlugin `pluginConfig` (set by the operator on each Engine)
2. `envoyfilter-waf-stats.yaml` (`stats_config.stats_tags` regex extractors)
3. Mesh or gateway `proxyStatsMatcher` inclusion for `waf_filter` stats
4. A Prometheus `PodMonitor` with `keepMetricRegex: waf_filter_.*` (see `hack/kind/coraza-monitoring-values.yaml`)

Verify on a gateway pod:

```bash
kubectl exec -n integration-tests deploy/coraza-gateway-istio -c istio-proxy -- \
  curl -s localhost:15090/stats/prometheus | grep waf_filter
```

See [coraza-proxy-wasm WAF metrics](https://github.com/corazawaf/coraza-proxy-wasm#waf-metrics).

## Test

```bash
kubectl port-forward svc/coraza-gateway-istio 8080:80
```

```bash
curl http://localhost:8080/                                  # normal request
curl -I "http://localhost:8080/?q=evilmonkey"                # blocked (rule 3001, 403)
curl "http://localhost:8080/?q=select+*+from+users"          # logged (rule 1001)
curl "http://localhost:8080/?q=<script>alert(1)</script>"    # logged (rule 2001)
```

Check gateway logs for Coraza output:

```bash
kubectl logs deploy/coraza-gateway-istio
```

## Cleanup

```bash
kubectl delete -f config/samples/
```
