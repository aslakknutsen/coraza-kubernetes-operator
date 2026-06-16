import * as React from 'react';
import {
  Card,
  CardHeader,
  CardTitle,
  CardBody,
  CardExpandableContent,
} from '@patternfly/react-core';
import { K8sResourceCommon } from '@openshift-console/dynamic-plugin-sdk';
import { ResourceYAML } from './ResourceYAML';

interface ResourceYAMLCardProps {
  resource: K8sResourceCommon;
}

export const ResourceYAMLCard: React.FC<ResourceYAMLCardProps> = ({ resource }) => {
  const [isExpanded, setIsExpanded] = React.useState(false);
  const cardId = 'resource-yaml';

  return (
    <Card id={cardId} isExpanded={isExpanded}>
      <CardHeader
        onExpand={() => setIsExpanded((expanded) => !expanded)}
        toggleButtonProps={{
          id: `${cardId}-toggle`,
          'aria-expanded': isExpanded,
          'aria-label': 'YAML',
        }}
      >
        <CardTitle>YAML</CardTitle>
      </CardHeader>
      <CardExpandableContent>
        <CardBody>
          <ResourceYAML resource={resource} />
        </CardBody>
      </CardExpandableContent>
    </Card>
  );
};
