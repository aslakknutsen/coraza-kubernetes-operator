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
  Bullseye,
  Spinner,
  Breadcrumb,
  BreadcrumbItem,
  List,
  ListItem,
} from '@patternfly/react-core';
import { RuleSetResource } from '../utils/types';
import { RuleSetModel } from '../utils/model';
import { getReadyStatus } from '../utils/status';
import { StatusLabel } from '../components/StatusLabel';
import { ConditionsTable } from '../components/ConditionsTable';
import { ResourceYAML } from '../components/ResourceYAML';

export default function RuleSetDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? 'default' : activeNamespace;

  const searchParams = new URLSearchParams(window.location.search);
  const namespace = searchParams.get('ns') ?? ns;

  const [ruleSet, loaded] = useK8sWatchResource<RuleSetResource>({
    groupVersionKind: { group: RuleSetModel.apiGroup!, version: RuleSetModel.apiVersion, kind: RuleSetModel.kind },
    name,
    namespace,
  });

  if (!loaded || !ruleSet) {
    return (
      <Bullseye>
        <Spinner />
      </Bullseye>
    );
  }

  return (
    <Page>
      <PageSection>
        <Breadcrumb>
          <BreadcrumbItem>
            <Link to="/coraza/rulesets">RuleSets</Link>
          </BreadcrumbItem>
          <BreadcrumbItem isActive>{name}</BreadcrumbItem>
        </Breadcrumb>
      </PageSection>
      <PageSection>
        <Title headingLevel="h1">
          {name}{' '}
          <StatusLabel status={getReadyStatus(ruleSet.status?.conditions)} />
        </Title>
        <span>{namespace}</span>
      </PageSection>

      <PageSection>
        <Card>
          <CardTitle>Rule Sources</CardTitle>
          <CardBody>
            {ruleSet.spec.rules && ruleSet.spec.rules.length > 0 ? (
              <List isPlain>
                {ruleSet.spec.rules.map((rule, i) => (
                  <ListItem key={i}>
                    <code>{rule.name}</code>
                  </ListItem>
                ))}
              </List>
            ) : (
              <span>No rule sources configured.</span>
            )}
          </CardBody>
        </Card>
      </PageSection>

      {ruleSet.spec.ruleData && (
        <PageSection>
          <Card>
            <CardTitle>Rule Data</CardTitle>
            <CardBody>
              <DescriptionList>
                <DescriptionListGroup>
                  <DescriptionListTerm>Secret</DescriptionListTerm>
                  <DescriptionListDescription>
                    <code>{ruleSet.spec.ruleData}</code>
                  </DescriptionListDescription>
                </DescriptionListGroup>
              </DescriptionList>
            </CardBody>
          </Card>
        </PageSection>
      )}

      {ruleSet.status?.conditions && ruleSet.status.conditions.length > 0 && (
        <PageSection>
          <Card>
            <CardTitle>Conditions</CardTitle>
            <CardBody>
              <ConditionsTable conditions={ruleSet.status.conditions} />
            </CardBody>
          </Card>
        </PageSection>
      )}

      <PageSection>
        <Card>
          <CardTitle>YAML</CardTitle>
          <CardBody>
            <ResourceYAML resource={ruleSet} />
          </CardBody>
        </Card>
      </PageSection>
    </Page>
  );
}
