import { Link, useParams } from 'react-router-dom-v5-compat';
import {
  useK8sWatchResource,
  useActiveNamespace,
} from '@openshift-console/dynamic-plugin-sdk';
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
import { RuleSourceResource } from '../utils/types';
import { RuleSourceModel } from '../utils/model';
import { getResourceStatus } from '../utils/status';
import { StatusLabel } from '../components/StatusLabel';
import { ConditionsTable } from '../components/ConditionsTable';
import { ResourceYAML } from '../components/ResourceYAML';

const VALIDATION_ANNOTATION = 'waf.k8s.coraza.io/rule-validation';

export default function RuleSourceDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? 'default' : activeNamespace;

  const searchParams = new URLSearchParams(window.location.search);
  const namespace = searchParams.get('ns') ?? ns;

  const [ruleSource, loaded, loadError] = useK8sWatchResource<RuleSourceResource>({
    groupVersionKind: {
      group: RuleSourceModel.apiGroup!,
      version: RuleSourceModel.apiVersion,
      kind: RuleSourceModel.kind,
    },
    name,
    namespace,
  });

  if (loadError) {
    return (
      <PageSection>
        <Title headingLevel="h1">Failed to load RuleSource</Title>
        <span>{loadError.message ?? String(loadError)}</span>
      </PageSection>
    );
  }

  if (!loaded || !ruleSource?.spec) {
    return (
      <Bullseye>
        <Spinner />
      </Bullseye>
    );
  }

  const { spec } = ruleSource;
  const validationAnnotation = ruleSource.metadata?.annotations?.[VALIDATION_ANNOTATION];

  return (
    <>
      <PageSection>
        <Breadcrumb>
          <BreadcrumbItem>
            <Link to="/coraza/rulesources">RuleSources</Link>
          </BreadcrumbItem>
          <BreadcrumbItem isActive>{name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>
      <PageSection>
        <Title headingLevel="h1">
          {name}{' '}
          <StatusLabel status={getResourceStatus(ruleSource.status?.conditions)} />
        </Title>
        <span>{namespace}</span>
      </PageSection>

      <PageSection>
        <Card>
          <CardTitle>Spec</CardTitle>
          <CardBody>
            <DescriptionList>
              <DescriptionListGroup>
                <DescriptionListTerm>Rules</DescriptionListTerm>
                <DescriptionListDescription>
                  <pre style={{ maxHeight: '20rem', overflow: 'auto', whiteSpace: 'pre-wrap' }}>
                    {spec.rules || '—'}
                  </pre>
                </DescriptionListDescription>
              </DescriptionListGroup>
              {validationAnnotation != null && (
                <DescriptionListGroup>
                  <DescriptionListTerm>Rule Validation</DescriptionListTerm>
                  <DescriptionListDescription>{validationAnnotation}</DescriptionListDescription>
                </DescriptionListGroup>
              )}
            </DescriptionList>
          </CardBody>
        </Card>
      </PageSection>

      {ruleSource.status?.conditions && ruleSource.status.conditions.length > 0 && (
        <PageSection>
          <Card>
            <CardTitle>Conditions</CardTitle>
            <CardBody>
              <ConditionsTable conditions={ruleSource.status.conditions} />
            </CardBody>
          </Card>
        </PageSection>
      )}

      <PageSection>
        <Card>
          <CardTitle>YAML</CardTitle>
          <CardBody>
            <ResourceYAML resource={ruleSource} />
          </CardBody>
        </Card>
      </PageSection>
    </>
  );
}
