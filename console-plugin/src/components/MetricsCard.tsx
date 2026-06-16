import { QueryBrowser } from '@openshift-console/dynamic-plugin-sdk';
import { Card, CardTitle, CardBody } from '@patternfly/react-core';
import { DEFAULT_GRAPH_TIMESPAN_MS, DEFAULT_POLL_INTERVAL_MS } from '../utils/promql';

export type MetricsCardProps = {
  title: string;
  queries: string[];
  namespace?: string;
  defaultTimespan?: number;
  isStack?: boolean;
  showLegend?: boolean;
};

export function MetricsCard({
  title,
  queries,
  namespace,
  defaultTimespan = DEFAULT_GRAPH_TIMESPAN_MS,
  isStack,
  showLegend,
}: MetricsCardProps) {
  return (
    <Card>
      <CardTitle>{title}</CardTitle>
      <CardBody>
        <QueryBrowser
          queries={queries}
          namespace={namespace}
          pollInterval={DEFAULT_POLL_INTERVAL_MS}
          defaultTimespan={defaultTimespan}
          isStack={isStack}
          showLegend={showLegend}
        />
      </CardBody>
    </Card>
  );
}
