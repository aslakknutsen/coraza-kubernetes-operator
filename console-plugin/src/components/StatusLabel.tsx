import * as React from 'react';
import { Label } from '@patternfly/react-core';
import {
  CheckCircleIcon,
  ExclamationCircleIcon,
  InProgressIcon,
  UnknownIcon,
} from '@patternfly/react-icons';
import { ReadyStatus } from '../utils/status';

const statusConfig: Record<ReadyStatus, { color: 'green' | 'red' | 'blue' | 'grey'; icon: React.ReactElement }> = {
  Ready: { color: 'green', icon: <CheckCircleIcon /> },
  'Not Ready': { color: 'red', icon: <ExclamationCircleIcon /> },
  Progressing: { color: 'blue', icon: <InProgressIcon /> },
  Unknown: { color: 'grey', icon: <UnknownIcon /> },
};

interface StatusLabelProps {
  status: ReadyStatus;
}

export const StatusLabel: React.FC<StatusLabelProps> = ({ status }) => {
  const cfg = statusConfig[status] ?? statusConfig.Unknown;
  return (
    <Label color={cfg.color} icon={cfg.icon}>
      {status}
    </Label>
  );
};
