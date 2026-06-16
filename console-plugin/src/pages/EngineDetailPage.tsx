import { useEffect, useRef } from 'react';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom-v5-compat';
import { useK8sWatchResource } from '@openshift-console/dynamic-plugin-sdk';
import {
  PageSection,
  Title,
  Card,
  CardTitle,
  CardBody,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Bullseye,
  Spinner,
  Breadcrumb,
  BreadcrumbItem,
} from '@patternfly/react-core';
import { EngineResource } from '../utils/types';
import { EngineModel } from '../utils/model';
import { getCondition, getResourceStatus } from '../utils/status';
import { useResourceNamespace } from '../utils/namespace';
import { StatusLabel } from '../components/StatusLabel';
import { DetailPageTabs } from '../components/DetailPageTabs';
import { ConditionsTable } from '../components/ConditionsTable';
import { ResourceYAMLCard } from '../components/ResourceYAMLCard';
import { MetricsCard } from '../components/MetricsCard';
import { engineQueries } from '../utils/promql';

const engineGVK = {
  group: EngineModel.apiGroup!,
  version: EngineModel.apiVersion,
  kind: EngineModel.kind,
};

type EngineDetailsTabProps = {
  engine: EngineResource;
  namespace: string;
};

function EngineDetailsTab({ engine, namespace }: EngineDetailsTabProps) {
  const spec = engine.spec;
  if (!spec) {
    return null;
  }

  const wasm = spec.driver?.wasm;
  const accepted = getCondition(engine.status?.conditions, 'Accepted');

  return (
    <>
      <PageSection>
        <Card>
          <CardTitle>Spec</CardTitle>
          <CardBody>
            <DescriptionList>
              <DescriptionListGroup>
                <DescriptionListTerm>RuleSet</DescriptionListTerm>
                <DescriptionListDescription>
                  <Link to={`/coraza/rulesets/${spec.ruleSet.name}?ns=${namespace}`}>
                    {spec.ruleSet.name}
                  </Link>
                </DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Target Provider</DescriptionListTerm>
                <DescriptionListDescription>{spec.target.provider ?? '—'}</DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Target Type</DescriptionListTerm>
                <DescriptionListDescription>{spec.target.type ?? '—'}</DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Target Name</DescriptionListTerm>
                <DescriptionListDescription>
                  <code>{spec.target.name ?? '—'}</code>
                </DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Driver</DescriptionListTerm>
                <DescriptionListDescription>{spec.driver?.type ?? '—'}</DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>WASM Image</DescriptionListTerm>
                <DescriptionListDescription>
                  <code>{wasm?.image ?? '—'}</code>
                </DescriptionListDescription>
              </DescriptionListGroup>
              {wasm?.imagePullSecret && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Image Pull Secret</DescriptionListTerm>
                  <DescriptionListDescription>
                    <code>{wasm.imagePullSecret}</code>
                  </DescriptionListDescription>
                </DescriptionListGroup>
              )}
              <DescriptionListGroup>
                <DescriptionListTerm>Failure Policy</DescriptionListTerm>
                <DescriptionListDescription>{spec.failurePolicy ?? '—'}</DescriptionListDescription>
              </DescriptionListGroup>
              {spec.ruleSetCacheServer?.pollIntervalSeconds != null && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Poll Interval</DescriptionListTerm>
                  <DescriptionListDescription>
                    {spec.ruleSetCacheServer.pollIntervalSeconds}s
                  </DescriptionListDescription>
                </DescriptionListGroup>
              )}
            </DescriptionList>
          </CardBody>
        </Card>
      </PageSection>

      {accepted && (
        <PageSection>
          <Card>
            <CardTitle>Accepted</CardTitle>
            <CardBody>
              <DescriptionList>
                <DescriptionListGroup>
                  <DescriptionListTerm>Status</DescriptionListTerm>
                  <DescriptionListDescription>{accepted.status}</DescriptionListDescription>
                </DescriptionListGroup>
                <DescriptionListGroup>
                  <DescriptionListTerm>Reason</DescriptionListTerm>
                  <DescriptionListDescription>{accepted.reason ?? '—'}</DescriptionListDescription>
                </DescriptionListGroup>
                {accepted.message && (
                  <DescriptionListGroup>
                    <DescriptionListTerm>Message</DescriptionListTerm>
                    <DescriptionListDescription>{accepted.message}</DescriptionListDescription>
                  </DescriptionListGroup>
                )}
              </DescriptionList>
            </CardBody>
          </Card>
        </PageSection>
      )}

      {engine.status?.conditions && engine.status.conditions.length > 0 && (
        <PageSection>
          <Card>
            <CardTitle>Conditions</CardTitle>
            <CardBody>
              <ConditionsTable conditions={engine.status.conditions} />
            </CardBody>
          </Card>
        </PageSection>
      )}

      <PageSection>
        <ResourceYAMLCard resource={engine} />
      </PageSection>
    </>
  );
}

type EngineMetricsTabProps = {
  name: string;
  namespace: string;
};

function EngineMetricsTab({ name, namespace }: EngineMetricsTabProps) {
  return (
    <>
      <PageSection>
        <MetricsCard
          title="WAF transaction rate"
          queries={[engineQueries.txRate(name, namespace)]}
          namespace={namespace}
        />
      </PageSection>
      <PageSection>
        <MetricsCard
          title="WAF interruptions by rule"
          queries={[engineQueries.interruptionsByRule(name, namespace)]}
          namespace={namespace}
          showLegend
        />
      </PageSection>
      <PageSection>
        <MetricsCard
          title="Engine conditions"
          queries={[engineQueries.conditionTimeline(name, namespace)]}
          namespace={namespace}
          showLegend
        />
      </PageSection>
    </>
  );
}

function engineDetailPath(name: string, namespace: string, tab: 'details' | 'metrics' = 'details'): string {
  const path =
    tab === 'metrics' ? `/coraza/engines/${name}/metrics` : `/coraza/engines/${name}`;
  return `${path}?ns=${encodeURIComponent(namespace)}`;
}

export default function EngineDetailPage() {
  const { name } = useParams<{ name: string }>();
  const namespaceHint = useResourceNamespace();
  const location = useLocation();
  const navigate = useNavigate();
  const lockedNamespace = useRef<string | undefined>();
  const watchNamespace = namespaceHint ?? lockedNamespace.current;
  const isMetricsTab = location.pathname.endsWith('/metrics');

  const [engines, loaded, loadError] = useK8sWatchResource<EngineResource[]>({
    groupVersionKind: engineGVK,
    isList: true,
    namespace: watchNamespace,
  });

  const matching = (engines ?? []).filter((e) => e.metadata?.name === name);
  const engine =
    matching.length === 1
      ? matching[0]
      : matching.find((e) => e.metadata?.namespace === watchNamespace);
  const namespace = engine?.metadata?.namespace ?? watchNamespace ?? '';

  if (namespace) {
    lockedNamespace.current = namespace;
  }

  const namespaceSearch = namespace ? `?ns=${encodeURIComponent(namespace)}` : location.search;

  useEffect(() => {
    if (!name || !namespace) {
      return;
    }
    const params = new URLSearchParams(location.search);
    const needsNs = !params.get('ns');
    if (needsNs) {
      params.set('ns', namespace);
    }
    const search = params.toString() ? `?${params.toString()}` : '';
    const legacyDetails = location.pathname.match(/\/coraza\/engines\/[^/]+\/details$/);
    if (legacyDetails) {
      navigate({ pathname: `/coraza/engines/${name}`, search }, { replace: true });
      return;
    }
    if (needsNs) {
      navigate({ pathname: location.pathname, search }, { replace: true });
    }
  }, [location.pathname, location.search, name, namespace, navigate]);

  if (loadError) {
    return (
      <PageSection>
        <Title headingLevel="h1">Failed to load Engine</Title>
        <span>{loadError.message ?? String(loadError)}</span>
      </PageSection>
    );
  }

  if (!loaded || !name) {
    return (
      <Bullseye>
        <Spinner />
      </Bullseye>
    );
  }

  if (matching.length > 1 && !watchNamespace) {
    return (
      <>
        <PageSection>
          <Title headingLevel="h1">Multiple Engines named {name}</Title>
        </PageSection>
        <PageSection>
          <table className="pf-v6-c-table pf-m-compact pf-m-grid-md" role="grid">
            <thead>
              <tr>
                <th>Namespace</th>
                <th>RuleSet</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {matching.map((e) => (
                <tr key={`${e.metadata?.namespace}-${e.metadata?.name}`}>
                  <td>
                    <Link to={engineDetailPath(name, e.metadata?.namespace ?? '')}>
                      {e.metadata?.namespace}
                    </Link>
                  </td>
                  <td>{e.spec?.ruleSet?.name ?? '—'}</td>
                  <td>
                    <StatusLabel
                      status={getResourceStatus(e.status?.conditions, { checkAccepted: true })}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </PageSection>
      </>
    );
  }

  if (!engine?.spec) {
    return (
      <PageSection>
        <Title headingLevel="h1">Engine not found</Title>
        <span>
          engines.waf.k8s.coraza.io &quot;{name}&quot; not found
          {namespaceHint ? ` in namespace ${namespaceHint}` : ''}.
        </span>
      </PageSection>
    );
  }

  return (
    <>
      <PageSection>
        <Breadcrumb>
          <BreadcrumbItem>
            <Link to="/coraza/engines">Engines</Link>
          </BreadcrumbItem>
          <BreadcrumbItem isActive>{name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>
      <PageSection>
        <Title headingLevel="h1">
          {name}{' '}
          <StatusLabel
            status={getResourceStatus(engine.status?.conditions, { checkAccepted: true })}
          />
        </Title>
        <span>{namespace}</span>
      </PageSection>

      <DetailPageTabs
        search={namespaceSearch}
        tabs={[
          {
            id: 'details',
            label: 'Details',
            pathname: `/coraza/engines/${name}`,
            isActive: (pathname) => !pathname.endsWith('/metrics'),
          },
          {
            id: 'metrics',
            label: 'Metrics',
            pathname: `/coraza/engines/${name}/metrics`,
            isActive: (pathname) => pathname.endsWith('/metrics'),
          },
        ]}
      />

      {isMetricsTab ? (
        <EngineMetricsTab name={name} namespace={namespace} />
      ) : (
        <EngineDetailsTab engine={engine} namespace={namespace} />
      )}
    </>
  );
}
