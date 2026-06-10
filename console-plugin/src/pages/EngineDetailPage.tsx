import { useParams } from 'react-router-dom';
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
import { StatusLabel } from '../components/StatusLabel';
import { ConditionsTable } from '../components/ConditionsTable';
import { ResourceYAML } from '../components/ResourceYAML';

export default function EngineDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? 'default' : activeNamespace;

  const searchParams = new URLSearchParams(window.location.search);
  const namespace = searchParams.get('ns') ?? ns;

  const [engine, loaded] = useK8sWatchResource<EngineResource>({
    groupVersionKind: { group: EngineModel.apiGroup!, version: EngineModel.apiVersion, kind: EngineModel.kind },
    name,
    namespace,
  });

  if (!loaded || !engine) {
    return (
      <Bullseye>
        <Spinner />
      </Bullseye>
    );
  }

  const wasm = engine.spec.driver?.wasm;
  const accepted = getCondition(engine.status?.conditions, 'Accepted');

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

      <PageSection>
        <Card>
          <CardTitle>Spec</CardTitle>
          <CardBody>
            <DescriptionList>
              <DescriptionListGroup>
                <DescriptionListTerm>RuleSet</DescriptionListTerm>
                <DescriptionListDescription>
                  <Link to={`/coraza/rulesets/${engine.spec.ruleSet.name}?ns=${namespace}`}>
                    {engine.spec.ruleSet.name}
                  </Link>
                </DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Target Provider</DescriptionListTerm>
                <DescriptionListDescription>{engine.spec.target?.provider ?? '—'}</DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Target Type</DescriptionListTerm>
                <DescriptionListDescription>{engine.spec.target?.type ?? '—'}</DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Target Name</DescriptionListTerm>
                <DescriptionListDescription>
                  <code>{engine.spec.target?.name ?? '—'}</code>
                </DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Driver</DescriptionListTerm>
                <DescriptionListDescription>
                  {engine.spec.driver?.type ?? '—'}
                </DescriptionListDescription>
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
                <DescriptionListDescription>{engine.spec.failurePolicy ?? '—'}</DescriptionListDescription>
              </DescriptionListGroup>
              {engine.spec.ruleSetCacheServer?.pollIntervalSeconds != null && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Poll Interval</DescriptionListTerm>
                  <DescriptionListDescription>
                    {engine.spec.ruleSetCacheServer.pollIntervalSeconds}s
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
        <Card>
          <CardTitle>YAML</CardTitle>
          <CardBody>
            <ResourceYAML resource={engine} />
          </CardBody>
        </Card>
      </PageSection>
    </>
  );
}
