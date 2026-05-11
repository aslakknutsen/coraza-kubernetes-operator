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
import { getReadyStatus } from '../utils/status';
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

  const driverType = engine.spec?.driver?.type ?? 'wasm';
  const wasmImage = engine.spec?.driver?.wasm?.image;

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
          <StatusLabel status={getReadyStatus(engine.status?.conditions)} />
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
                  {engine.spec?.ruleSet?.name ? (
                    <Link to={`/coraza/rulesets/${engine.spec.ruleSet.name}?ns=${namespace}`}>
                      {engine.spec.ruleSet.name}
                    </Link>
                  ) : '-'}
                </DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Target</DescriptionListTerm>
                <DescriptionListDescription>
                  {engine.spec?.target?.type ?? '-'}{engine.spec?.target?.name ? ` / ${engine.spec.target.name}` : ''}
                </DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Failure Policy</DescriptionListTerm>
                <DescriptionListDescription>{engine.spec?.failurePolicy ?? 'fail'}</DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Driver</DescriptionListTerm>
                <DescriptionListDescription>{driverType}</DescriptionListDescription>
              </DescriptionListGroup>
              {wasmImage && (
                <DescriptionListGroup>
                  <DescriptionListTerm>WASM Image</DescriptionListTerm>
                  <DescriptionListDescription>
                    <code>{wasmImage}</code>
                  </DescriptionListDescription>
                </DescriptionListGroup>
              )}
              {engine.spec?.driver?.wasm?.imagePullSecret && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Image Pull Secret</DescriptionListTerm>
                  <DescriptionListDescription>
                    <code>{engine.spec.driver!.wasm!.imagePullSecret}</code>
                  </DescriptionListDescription>
                </DescriptionListGroup>
              )}
              {engine.spec?.ruleSetCacheServer != null && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Poll Interval</DescriptionListTerm>
                  <DescriptionListDescription>
                    {engine.spec.ruleSetCacheServer.pollIntervalSeconds ?? 15}s
                  </DescriptionListDescription>
                </DescriptionListGroup>
              )}
            </DescriptionList>
          </CardBody>
        </Card>
      </PageSection>

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
