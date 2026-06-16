import * as React from 'react';
import { Condition } from '../utils/types';

interface ConditionsTableProps {
  conditions: Condition[];
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
            <td>{c.status}</td>
            <td>{c.reason ?? '-'}</td>
            <td>{c.message ?? '-'}</td>
            <td>{c.lastTransitionTime ?? '-'}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
};
