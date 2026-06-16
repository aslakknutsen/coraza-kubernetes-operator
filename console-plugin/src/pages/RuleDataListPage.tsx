import {
  useK8sWatchResource,
  useActiveNamespace,
} from '@openshift-console/dynamic-plugin-sdk';
import { Link } from 'react-router-dom-v5-compat';
import {
  PageSection,
  Title,
  Bullseye,
  Spinner,
  EmptyState,
  EmptyStateBody,
} from '@patternfly/react-core';
import { RuleDataResource } from '../utils/types';
import { RuleDataModel } from '../utils/model';
import { getResourceStatus } from '../utils/status';
import { StatusLabel } from '../components/StatusLabel';

export default function RuleDataListPage() {
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? undefined : activeNamespace;

  const [ruleDataList, loaded] = useK8sWatchResource<RuleDataResource[]>({
    groupVersionKind: {
      group: RuleDataModel.apiGroup!,
      version: RuleDataModel.apiVersion,
      kind: RuleDataModel.kind,
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

  const items = ruleDataList ?? [];

  return (
    <>
      <PageSection>
        <Title headingLevel="h1">RuleData</Title>
      </PageSection>
      <PageSection>
        {items.length === 0 ? (
          <EmptyState>
            <EmptyStateBody>No RuleData found. Create a RuleData resource to store @pmFromFile data files.</EmptyStateBody>
          </EmptyState>
        ) : (
          <table className="pf-v6-c-table pf-m-compact pf-m-grid-md" role="grid">
            <thead>
              <tr>
                <th>Name</th>
                <th>Namespace</th>
                <th>Files</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {items.map((r) => (
                <tr key={`${r.metadata?.namespace}-${r.metadata?.name}`}>
                  <td>
                    <Link to={`/coraza/ruledata/${r.metadata?.name}?ns=${r.metadata?.namespace}`}>
                      {r.metadata?.name}
                    </Link>
                  </td>
                  <td>{r.metadata?.namespace}</td>
                  <td>{Object.keys(r.spec.files ?? {}).length}</td>
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
