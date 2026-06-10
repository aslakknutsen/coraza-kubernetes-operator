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
import { ResourceYAML } from '../components/ResourceYAML';

export default function RuleDataDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? 'default' : activeNamespace;

  const searchParams = new URLSearchParams(window.location.search);
  const namespace = searchParams.get('ns') ?? ns;

  const [ruleData, loaded] = useK8sWatchResource<RuleDataResource>({
    groupVersionKind: {
      group: RuleDataModel.apiGroup!,
      version: RuleDataModel.apiVersion,
      kind: RuleDataModel.kind,
    },
    name,
    namespace,
  });

  if (!loaded || !ruleData) {
    return (
      <Bullseye>
        <Spinner />
      </Bullseye>
    );
  }

  const fileNames = Object.keys(ruleData.spec.files ?? {});

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
        <Card>
          <CardTitle>YAML</CardTitle>
          <CardBody>
            <ResourceYAML resource={ruleData} />
          </CardBody>
        </Card>
      </PageSection>
    </>
  );
}
