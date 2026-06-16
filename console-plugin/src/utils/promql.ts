/** Default graph timespan: 30 minutes. */
export const DEFAULT_GRAPH_TIMESPAN_MS = 30 * 60 * 1000;

export const DEFAULT_POLL_INTERVAL_MS = 30_000;

/** Namespace passed to QueryBrowser; undefined when viewing all namespaces. */
export function queryBrowserNamespace(
  activeNamespace: string,
): string | undefined {
  if (activeNamespace === '#ALL_NS#') {
    return undefined;
  }
  return activeNamespace;
}

export const overviewQueries = {
  enginesNotReady: () =>
    'coraza:engines_not_ready:count or count(coraza_engine_condition{condition="Ready"} != 1)',
  ruleSetsNotReady: () =>
    'coraza:rulesets_not_ready:count or count(coraza_ruleset_condition{condition="Ready"} != 1)',
  cacheUtilization: () =>
    'coraza_cache_size_bytes / coraza_cache_config_max_size_bytes',
  topWafInterruptions: (namespace?: string) => {
    const selector = namespace ? `{namespace="${namespace}"}` : '';
    return `topk(5, sum by (rule_id) (rate(waf_filter_tx_interruptions${selector}[5m])))`;
  },
};

export const engineQueries = {
  txRate: (name: string, namespace: string) =>
    `rate(waf_filter_tx_total{engine="${name}",namespace="${namespace}"}[5m])`,
  interruptionsByRule: (name: string, namespace: string) =>
    `sum by (rule_id, phase) (rate(waf_filter_tx_interruptions{engine="${name}",namespace="${namespace}"}[5m]))`,
  conditionTimeline: (name: string, namespace: string) =>
    `coraza_engine_condition{namespace="${namespace}",name="${name}"}`,
};

export const ruleSetQueries = {
  conditionTimeline: (name: string, namespace: string) =>
    `coraza_ruleset_condition{namespace="${namespace}",name="${name}"}`,
};
