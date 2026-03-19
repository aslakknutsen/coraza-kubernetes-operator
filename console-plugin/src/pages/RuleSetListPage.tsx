import {
  useK8sWatchResource,
  useActiveNamespace,
} from '@openshift-console/dynamic-plugin-sdk';
import { Link } from 'react-router-dom';
import {
  PageSection,
  Title,
  Bullseye,
  Spinner,
  EmptyState,
  EmptyStateBody,
} from '@patternfly/react-core';
import { RuleSetResource } from '../utils/types';
import { RuleSetModel } from '../utils/model';
import { getReadyStatus } from '../utils/status';
import { StatusLabel } from '../components/StatusLabel';

export default function RuleSetListPage() {
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? undefined : activeNamespace;

  const [ruleSets, loaded] = useK8sWatchResource<RuleSetResource[]>({
    groupVersionKind: { group: RuleSetModel.apiGroup!, version: RuleSetModel.apiVersion, kind: RuleSetModel.kind },
    isList: true,
    namespace: ns,
  });

  if (!loaded) {
    return (
      <Bullseye>
        <Spinner />
      </Bullseye>
    );
  }

  const items = ruleSets ?? [];

  return (
    <>
      <PageSection>
        <Title headingLevel="h1">RuleSets</Title>
      </PageSection>
      <PageSection>
        {items.length === 0 ? (
          <EmptyState>
            <EmptyStateBody>No RuleSets found. Create a RuleSet resource to define WAF rules.</EmptyStateBody>
          </EmptyState>
        ) : (
          <table className="pf-v6-c-table pf-m-compact pf-m-grid-md" role="grid">
            <thead>
              <tr>
                <th>Name</th>
                <th>Namespace</th>
                <th>Rule Sources</th>
                <th>Rule Data</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {items.map((r) => (
                <tr key={`${r.metadata?.namespace}-${r.metadata?.name}`}>
                  <td>
                    <Link to={`/coraza/rulesets/${r.metadata?.name}?ns=${r.metadata?.namespace}`}>
                      {r.metadata?.name}
                    </Link>
                  </td>
                  <td>{r.metadata?.namespace}</td>
                  <td>{r.spec.rules?.length ?? 0}</td>
                  <td>{r.spec.ruleData ? <code>{r.spec.ruleData}</code> : '—'}</td>
                  <td><StatusLabel status={getReadyStatus(r.status?.conditions)} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </PageSection>
    </>
  );
}
