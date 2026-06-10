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
import { EngineResource, RuleSetResource, RuleSourceResource, RuleDataResource } from '../utils/types';
import { EngineModel, RuleSetModel, RuleSourceModel, RuleDataModel } from '../utils/model';
import { getResourceStatus } from '../utils/status';
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

  const [ruleSources, ruleSourcesLoaded] = useK8sWatchResource<RuleSourceResource[]>({
    groupVersionKind: {
      group: RuleSourceModel.apiGroup!,
      version: RuleSourceModel.apiVersion,
      kind: RuleSourceModel.kind,
    },
    isList: true,
    namespace: ns,
  });

  const [ruleDataList, ruleDataLoaded] = useK8sWatchResource<RuleDataResource[]>({
    groupVersionKind: {
      group: RuleDataModel.apiGroup!,
      version: RuleDataModel.apiVersion,
      kind: RuleDataModel.kind,
    },
    isList: true,
    namespace: ns,
  });

  if (!enginesLoaded || !ruleSetsLoaded || !ruleSourcesLoaded || !ruleDataLoaded) {
    return (
      <Bullseye>
        <Spinner />
      </Bullseye>
    );
  }

  const engineList = engines ?? [];
  const ruleSetList = ruleSets ?? [];
  const ruleSourceList = ruleSources ?? [];
  const ruleDataItems = ruleDataList ?? [];

  const hasResources =
    engineList.length > 0 ||
    ruleSetList.length > 0 ||
    ruleSourceList.length > 0 ||
    ruleDataItems.length > 0;

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
          <GalleryItem>
            <Card>
              <CardTitle>RuleSources</CardTitle>
              <CardBody>
                <div style={{ fontSize: '2rem', fontWeight: 700 }}>{ruleSourceList.length}</div>
              </CardBody>
            </Card>
          </GalleryItem>
          <GalleryItem>
            <Card>
              <CardTitle>RuleData</CardTitle>
              <CardBody>
                <div style={{ fontSize: '2rem', fontWeight: 700 }}>{ruleDataItems.length}</div>
              </CardBody>
            </Card>
          </GalleryItem>
        </Gallery>
      </PageSection>
      <PageSection>
        <Card>
          <CardTitle>Recent Resources</CardTitle>
          <CardBody>
            {!hasResources ? (
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
                  {engineList.map((e) => (
                    <tr key={`engine-${e.metadata?.namespace}-${e.metadata?.name}`}>
                      <td>Engine</td>
                      <td>
                        <Link to={`/coraza/engines/${e.metadata?.name}?ns=${e.metadata?.namespace}`}>
                          {e.metadata?.name}
                        </Link>
                      </td>
                      <td>{e.metadata?.namespace}</td>
                      <td>
                        <StatusLabel
                          status={getResourceStatus(e.status?.conditions, { checkAccepted: true })}
                        />
                      </td>
                    </tr>
                  ))}
                  {ruleSetList.map((r) => (
                    <tr key={`ruleset-${r.metadata?.namespace}-${r.metadata?.name}`}>
                      <td>RuleSet</td>
                      <td>
                        <Link to={`/coraza/rulesets/${r.metadata?.name}?ns=${r.metadata?.namespace}`}>
                          {r.metadata?.name}
                        </Link>
                      </td>
                      <td>{r.metadata?.namespace}</td>
                      <td><StatusLabel status={getResourceStatus(r.status?.conditions)} /></td>
                    </tr>
                  ))}
                  {ruleSourceList.map((r) => (
                    <tr key={`rulesource-${r.metadata?.namespace}-${r.metadata?.name}`}>
                      <td>RuleSource</td>
                      <td>
                        <Link to={`/coraza/rulesources/${r.metadata?.name}?ns=${r.metadata?.namespace}`}>
                          {r.metadata?.name}
                        </Link>
                      </td>
                      <td>{r.metadata?.namespace}</td>
                      <td><StatusLabel status={getResourceStatus(r.status?.conditions)} /></td>
                    </tr>
                  ))}
                  {ruleDataItems.map((r) => (
                    <tr key={`ruledata-${r.metadata?.namespace}-${r.metadata?.name}`}>
                      <td>RuleData</td>
                      <td>
                        <Link to={`/coraza/ruledata/${r.metadata?.name}?ns=${r.metadata?.namespace}`}>
                          {r.metadata?.name}
                        </Link>
                      </td>
                      <td>{r.metadata?.namespace}</td>
                      <td><StatusLabel status={getResourceStatus(r.status?.conditions)} /></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </CardBody>
        </Card>
      </PageSection>
    </>
  );
}
