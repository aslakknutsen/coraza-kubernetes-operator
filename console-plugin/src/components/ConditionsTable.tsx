import * as React from 'react';
import { Label } from '@patternfly/react-core';
import {
  CheckCircleIcon,
  ExclamationCircleIcon,
  UnknownIcon,
} from '@patternfly/react-icons';
import { Condition } from '../utils/types';

interface ConditionsTableProps {
  conditions: Condition[];
}

function conditionStatusLabel(status: string): React.ReactElement {
  switch (status) {
    case 'True':
      return <Label color="green" icon={<CheckCircleIcon />}>True</Label>;
    case 'False':
      return <Label color="red" icon={<ExclamationCircleIcon />}>False</Label>;
    default:
      return <Label color="grey" icon={<UnknownIcon />}>{status || 'Unknown'}</Label>;
  }
}

export const ConditionsTable: React.FC<ConditionsTableProps> = ({ conditions }) => {
  if (!conditions || conditions.length === 0) return null;

  return (
    <table className="pf-v6-c-table pf-m-compact pf-m-grid-md" role="grid">
      <thead>
        <tr>
          <th>Type</th>
          <th>Status</th>
          <th>Reason</th>
          <th>Message</th>
          <th>Last Transition</th>
        </tr>
      </thead>
      <tbody>
        {conditions.map((c) => (
          <tr key={c.type}>
            <td>{c.type}</td>
            <td>{conditionStatusLabel(c.status)}</td>
            <td>{c.reason ?? '-'}</td>
            <td>{c.message ?? '-'}</td>
            <td>{c.lastTransitionTime ?? '-'}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
};
