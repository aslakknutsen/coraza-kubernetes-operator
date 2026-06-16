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
import { RuleSetResource } from '../utils/types';
import { RuleSetModel } from '../utils/model';
import { getResourceStatus } from '../utils/status';
import { StatusLabel } from '../components/StatusLabel';
import { ConditionsTable } from '../components/ConditionsTable';
import { ResourceYAMLCard } from '../components/ResourceYAMLCard';

export default function RuleSetDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [activeNamespace] = useActiveNamespace();
  const ns = activeNamespace === '#ALL_NS#' ? 'default' : activeNamespace;

  const searchParams = new URLSearchParams(window.location.search);
  const namespace = searchParams.get('ns') ?? ns;

  const [ruleSet, loaded, loadError] = useK8sWatchResource<RuleSetResource>({
    groupVersionKind: { group: RuleSetModel.apiGroup!, version: RuleSetModel.apiVersion, kind: RuleSetModel.kind },
    name,
    namespace,
  });

  if (loadError) {
    return (
      <PageSection>
        <Title headingLevel="h1">Failed to load RuleSet</Title>
        <span>{loadError.message ?? String(loadError)}</span>
      </PageSection>
    );
  }

  if (!loaded || !ruleSet?.spec) {
    return (
      <Bullseye>
        <Spinner />
      </Bullseye>
    );
  }

  const { spec } = ruleSet;

  return (
    <>
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
          <StatusLabel status={getResourceStatus(ruleSet.status?.conditions)} />
        </Title>
        <span>{namespace}</span>
      </PageSection>

      <PageSection>
        <Card>
          <CardTitle>Rule Sources</CardTitle>
          <CardBody>
            {spec.sources && spec.sources.length > 0 ? (
              <List isPlain>
                {spec.sources.map((source) => (
                  <ListItem key={source.name}>
                    <Link to={`/coraza/rulesources/${source.name}?ns=${namespace}`}>
                      <code>{source.name}</code>
                    </Link>
                  </ListItem>
                ))}
              </List>
            ) : (
              <span>No rule sources configured.</span>
            )}
          </CardBody>
        </Card>
      </PageSection>

      <PageSection>
        <Card>
          <CardTitle>Rule Data</CardTitle>
          <CardBody>
            {spec.data && spec.data.length > 0 ? (
              <List isPlain>
                {spec.data.map((data) => (
                  <ListItem key={data.name}>
                    <Link to={`/coraza/ruledata/${data.name}?ns=${namespace}`}>
                      <code>{data.name}</code>
                    </Link>
                  </ListItem>
                ))}
              </List>
            ) : (
              <span>No rule data configured.</span>
            )}
          </CardBody>
        </Card>
      </PageSection>

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
        <ResourceYAMLCard resource={ruleSet} />
      </PageSection>
    </>
  );
}
