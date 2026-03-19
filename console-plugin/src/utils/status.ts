import { Condition } from './types';

export function getReadyCondition(conditions?: Condition[]): Condition | undefined {
  return conditions?.find((c) => c.type === 'Ready');
}

export type ReadyStatus = 'Ready' | 'Not Ready' | 'Progressing' | 'Unknown';

export function getReadyStatus(conditions?: Condition[]): ReadyStatus {
  const ready = getReadyCondition(conditions);
  if (!ready) return 'Unknown';
  if (ready.status === 'True') return 'Ready';
  if (ready.reason === 'Progressing') return 'Progressing';
  if (ready.status === 'False') return 'Not Ready';
  return 'Unknown';
}
