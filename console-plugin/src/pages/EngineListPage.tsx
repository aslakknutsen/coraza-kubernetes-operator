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
import { EngineResource } from '../utils/types';
import { EngineModel } from '../utils/model';
import { getReadyStatus } from '../utils/status';
import { StatusLabel } from '../components/StatusLabel';

export default function EngineListPage() {
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? undefined : activeNamespace;

  const [engines, loaded] = useK8sWatchResource<EngineResource[]>({
    groupVersionKind: { group: EngineModel.apiGroup!, version: EngineModel.apiVersion, kind: EngineModel.kind },
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

  const items = engines ?? [];

  return (
    <>
      <PageSection>
        <Title headingLevel="h1">Engines</Title>
      </PageSection>
      <PageSection>
        {items.length === 0 ? (
          <EmptyState>
            <EmptyStateBody>No Engines found. Create an Engine resource to deploy a Coraza WAF instance.</EmptyStateBody>
          </EmptyState>
        ) : (
          <table className="pf-v6-c-table pf-m-compact pf-m-grid-md" role="grid">
            <thead>
              <tr>
                <th>Name</th>
                <th>Namespace</th>
                <th>RuleSet</th>
                <th>Failure Policy</th>
                <th>Gateways</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {items.map((e) => (
                  <tr key={`${e.metadata?.namespace}-${e.metadata?.name}`}>
                    <td>
                      <Link to={`/coraza/engines/${e.metadata?.name}?ns=${e.metadata?.namespace}`}>
                        {e.metadata?.name}
                      </Link>
                    </td>
                    <td>{e.metadata?.namespace}</td>
                    <td>
                      <Link to={`/coraza/rulesets/${e.spec.ruleSet.name}?ns=${e.metadata?.namespace}`}>
                        {e.spec.ruleSet.name}
                      </Link>
                    </td>
                    <td>{e.spec.failurePolicy}</td>
                    <td>{e.status?.gateways?.length ?? 0}</td>
                    <td><StatusLabel status={getReadyStatus(e.status?.conditions)} /></td>
                  </tr>
              ))}
            </tbody>
          </table>
        )}
      </PageSection>
    </>
  );
}
