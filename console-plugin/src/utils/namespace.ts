import { useActiveNamespace } from '@openshift-console/dynamic-plugin-sdk';
import { useLocation } from 'react-router-dom-v5-compat';

/** Namespace from ?ns= query param, else the console project namespace (not "all namespaces"). */
export function useResourceNamespace(): string | undefined {
  const [activeNamespace] = useActiveNamespace();
  const location = useLocation();
  const fromQuery = new URLSearchParams(location.search).get('ns');
  if (fromQuery) {
    return fromQuery;
  }
  if (activeNamespace && activeNamespace !== '#ALL_NS#') {
    return activeNamespace;
  }
  return undefined;
}
