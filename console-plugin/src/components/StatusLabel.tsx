import * as React from 'react';
import { Label } from '@patternfly/react-core';
import {
  CheckCircleIcon,
  ExclamationCircleIcon,
  InProgressIcon,
  UnknownIcon,
  BanIcon,
} from '@patternfly/react-icons';
import { ResourceStatus } from '../utils/status';

const statusConfig: Record<ResourceStatus, { color: 'green' | 'red' | 'blue' | 'grey' | 'orange'; icon: React.ReactElement }> = {
  Ready: { color: 'green', icon: <CheckCircleIcon /> },
  'Not Ready': { color: 'red', icon: <ExclamationCircleIcon /> },
  Progressing: { color: 'blue', icon: <InProgressIcon /> },
  Degraded: { color: 'red', icon: <ExclamationCircleIcon /> },
  Rejected: { color: 'orange', icon: <BanIcon /> },
  Unknown: { color: 'grey', icon: <UnknownIcon /> },
};

interface StatusLabelProps {
  status: ResourceStatus;
}

export const StatusLabel: React.FC<StatusLabelProps> = ({ status }) => {
  const cfg = statusConfig[status] ?? statusConfig.Unknown;
  return (
    <Label color={cfg.color} icon={cfg.icon}>
      {status}
    </Label>
  );
};
