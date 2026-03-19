import * as React from 'react';
import { useParams } from 'react-router-dom';
import {
  useK8sWatchResource,
  useActiveNamespace,
} from '@openshift-console/dynamic-plugin-sdk';
import { Link } from 'react-router-dom';
import {
  Page,
  PageSection,
  Title,
  Card,
  CardTitle,
  CardBody,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Label,
  LabelGroup,
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

  const wasm = engine.spec.driver.istio?.wasm;
  const matchLabels = wasm?.workloadSelector?.matchLabels ?? {};
  const gateways = engine.status?.gateways ?? [];

  return (
    <Page>
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
                  <Link to={`/coraza/rulesets/${engine.spec.ruleSet.name}?ns=${namespace}`}>
                    {engine.spec.ruleSet.name}
                  </Link>
                </DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Driver</DescriptionListTerm>
                <DescriptionListDescription>
                  Istio / {wasm?.mode ?? '-'}
                </DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>WASM Image</DescriptionListTerm>
                <DescriptionListDescription>
                  <code>{wasm?.image ?? '-'}</code>
                </DescriptionListDescription>
              </DescriptionListGroup>
              <DescriptionListGroup>
                <DescriptionListTerm>Failure Policy</DescriptionListTerm>
                <DescriptionListDescription>{engine.spec.failurePolicy}</DescriptionListDescription>
              </DescriptionListGroup>
              {Object.keys(matchLabels).length > 0 && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Workload Selector</DescriptionListTerm>
                  <DescriptionListDescription>
                    <LabelGroup>
                      {Object.entries(matchLabels).map(([k, v]) => (
                        <Label key={k}>{k}={v}</Label>
                      ))}
                    </LabelGroup>
                  </DescriptionListDescription>
                </DescriptionListGroup>
              )}
              {wasm?.ruleSetCacheServer && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Poll Interval</DescriptionListTerm>
                  <DescriptionListDescription>
                    {wasm.ruleSetCacheServer.pollIntervalSeconds}s
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

      {gateways.length > 0 && (
        <PageSection>
          <Card>
            <CardTitle>Matched Gateways</CardTitle>
            <CardBody>
              <LabelGroup>
                {gateways.map((g) => (
                  <Label key={g.name}>{g.name}</Label>
                ))}
              </LabelGroup>
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
    </Page>
  );
}
