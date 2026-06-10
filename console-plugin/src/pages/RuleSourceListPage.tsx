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
import { RuleSourceResource } from '../utils/types';
import { RuleSourceModel } from '../utils/model';
import { getResourceStatus } from '../utils/status';
import { StatusLabel } from '../components/StatusLabel';

export default function RuleSourceListPage() {
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? undefined : activeNamespace;

  const [ruleSources, loaded] = useK8sWatchResource<RuleSourceResource[]>({
    groupVersionKind: {
      group: RuleSourceModel.apiGroup!,
      version: RuleSourceModel.apiVersion,
      kind: RuleSourceModel.kind,
    },
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

  const items = ruleSources ?? [];

  return (
    <>
      <PageSection>
        <Title headingLevel="h1">RuleSources</Title>
      </PageSection>
      <PageSection>
        {items.length === 0 ? (
          <EmptyState>
            <EmptyStateBody>No RuleSources found. Create a RuleSource resource to store SecLang rules.</EmptyStateBody>
          </EmptyState>
        ) : (
          <table className="pf-v6-c-table pf-m-compact pf-m-grid-md" role="grid">
            <thead>
              <tr>
                <th>Name</th>
                <th>Namespace</th>
                <th>Rules Size</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {items.map((r) => (
                <tr key={`${r.metadata?.namespace}-${r.metadata?.name}`}>
                  <td>
                    <Link to={`/coraza/rulesources/${r.metadata?.name}?ns=${r.metadata?.namespace}`}>
                      {r.metadata?.name}
                    </Link>
                  </td>
                  <td>{r.metadata?.namespace}</td>
                  <td>{r.spec.rules?.length ?? 0} chars</td>
                  <td><StatusLabel status={getResourceStatus(r.status?.conditions)} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </PageSection>
    </>
  );
}
