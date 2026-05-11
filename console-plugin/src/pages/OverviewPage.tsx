import {
  useK8sWatchResource,
  useActiveNamespace,
} from '@openshift-console/dynamic-plugin-sdk';
import { Link } from 'react-router-dom';
import {
  PageSection,
  Title,
  Card,
  CardTitle,
  CardBody,
  Gallery,
  GalleryItem,
  Bullseye,
  Spinner,
  EmptyState,
  EmptyStateBody,
} from '@patternfly/react-core';
import { EngineResource, RuleSetResource } from '../utils/types';
import { EngineModel, RuleSetModel } from '../utils/model';
import { getReadyStatus } from '../utils/status';
import { StatusLabel } from '../components/StatusLabel';

export default function OverviewPage() {
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? undefined : activeNamespace;

  const [engines, enginesLoaded] = useK8sWatchResource<EngineResource[]>({
    groupVersionKind: { group: EngineModel.apiGroup!, version: EngineModel.apiVersion, kind: EngineModel.kind },
    isList: true,
    namespace: ns,
  });

  const [ruleSets, ruleSetsLoaded] = useK8sWatchResource<RuleSetResource[]>({
    groupVersionKind: { group: RuleSetModel.apiGroup!, version: RuleSetModel.apiVersion, kind: RuleSetModel.kind },
    isList: true,
    namespace: ns,
  });

  if (!enginesLoaded || !ruleSetsLoaded) {
    return (
      <Bullseye>
        <Spinner />
      </Bullseye>
    );
  }

  const engineList = engines ?? [];
  const ruleSetList = ruleSets ?? [];

  type TaggedResource = { kind: string; resource: EngineResource | RuleSetResource };
  const allResources: TaggedResource[] = [
    ...engineList.map((e) => ({ kind: 'Engine' as const, resource: e })),
    ...ruleSetList.map((r) => ({ kind: 'RuleSet' as const, resource: r })),
  ].sort((a, b) => {
    const ta = a.resource.metadata?.creationTimestamp ?? '';
    const tb = b.resource.metadata?.creationTimestamp ?? '';
    return tb.localeCompare(ta);
  });

  return (
    <>
      <PageSection>
        <Title headingLevel="h1">Coraza WAF Overview</Title>
      </PageSection>
      <PageSection>
        <Gallery hasGutter>
          <GalleryItem>
            <Card>
              <CardTitle>Engines</CardTitle>
              <CardBody>
                <div style={{ fontSize: '2rem', fontWeight: 700 }}>{engineList.length}</div>
              </CardBody>
            </Card>
          </GalleryItem>
          <GalleryItem>
            <Card>
              <CardTitle>RuleSets</CardTitle>
              <CardBody>
                <div style={{ fontSize: '2rem', fontWeight: 700 }}>{ruleSetList.length}</div>
              </CardBody>
            </Card>
          </GalleryItem>
        </Gallery>
      </PageSection>
      <PageSection>
        <Card>
          <CardTitle>Recent Resources</CardTitle>
          <CardBody>
            {allResources.length === 0 ? (
              <EmptyState>
                <EmptyStateBody>No Coraza resources found in this namespace.</EmptyStateBody>
              </EmptyState>
            ) : (
              <table className="pf-v6-c-table pf-m-compact pf-m-grid-md" role="grid">
                <thead>
                  <tr>
                    <th>Kind</th>
                    <th>Name</th>
                    <th>Namespace</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {allResources.map(({ kind, resource: r }) => {
                    const route = kind === 'Engine' ? 'engines' : 'rulesets';
                    const status = getReadyStatus(
                      (r as EngineResource).status?.conditions ??
                      (r as RuleSetResource).status?.conditions,
                    );
                    return (
                      <tr key={`${kind}-${r.metadata?.namespace}-${r.metadata?.name}`}>
                        <td>{kind}</td>
                        <td>
                          <Link to={`/coraza/${route}/${r.metadata?.name}?ns=${r.metadata?.namespace}`}>
                            {r.metadata?.name}
                          </Link>
                        </td>
                        <td>{r.metadata?.namespace}</td>
                        <td><StatusLabel status={status} /></td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </CardBody>
        </Card>
      </PageSection>
    </>
  );
}
