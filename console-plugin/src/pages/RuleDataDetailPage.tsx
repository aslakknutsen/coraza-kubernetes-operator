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
  Bullseye,
  Spinner,
  Breadcrumb,
  BreadcrumbItem,
  List,
  ListItem,
} from '@patternfly/react-core';
import { RuleDataResource } from '../utils/types';
import { RuleDataModel } from '../utils/model';
import { getResourceStatus } from '../utils/status';
import { StatusLabel } from '../components/StatusLabel';
import { ConditionsTable } from '../components/ConditionsTable';
import { ResourceYAMLCard } from '../components/ResourceYAMLCard';

export default function RuleDataDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? 'default' : activeNamespace;

  const searchParams = new URLSearchParams(window.location.search);
  const namespace = searchParams.get('ns') ?? ns;

  const [ruleData, loaded, loadError] = useK8sWatchResource<RuleDataResource>({
    groupVersionKind: {
      group: RuleDataModel.apiGroup!,
      version: RuleDataModel.apiVersion,
      kind: RuleDataModel.kind,
    },
    name,
    namespace,
  });

  if (loadError) {
    return (
      <PageSection>
        <Title headingLevel="h1">Failed to load RuleData</Title>
        <span>{loadError.message ?? String(loadError)}</span>
      </PageSection>
    );
  }

  if (!loaded || !ruleData?.spec) {
    return (
      <Bullseye>
        <Spinner />
      </Bullseye>
    );
  }

  const { spec } = ruleData;
  const fileNames = Object.keys(spec.files ?? {});

  return (
    <>
      <PageSection>
        <Breadcrumb>
          <BreadcrumbItem>
            <Link to="/coraza/ruledata">RuleData</Link>
          </BreadcrumbItem>
          <BreadcrumbItem isActive>{name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>
      <PageSection>
        <Title headingLevel="h1">
          {name}{' '}
          <StatusLabel status={getResourceStatus(ruleData.status?.conditions)} />
        </Title>
        <span>{namespace}</span>
      </PageSection>

      <PageSection>
        <Card>
          <CardTitle>Files</CardTitle>
          <CardBody>
            {fileNames.length > 0 ? (
              <List isPlain>
                {fileNames.map((fileName) => (
                  <ListItem key={fileName}>
                    <code>{fileName}</code>
                  </ListItem>
                ))}
              </List>
            ) : (
              <span>No files configured.</span>
            )}
          </CardBody>
        </Card>
      </PageSection>

      {ruleData.status?.conditions && ruleData.status.conditions.length > 0 && (
        <PageSection>
          <Card>
            <CardTitle>Conditions</CardTitle>
            <CardBody>
              <ConditionsTable conditions={ruleData.status.conditions} />
            </CardBody>
          </Card>
        </PageSection>
      )}

      <PageSection>
        <ResourceYAMLCard resource={ruleData} />
      </PageSection>
    </>
  );
}
