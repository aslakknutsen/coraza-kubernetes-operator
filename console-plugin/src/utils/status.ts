import { Condition } from './types';

export function getCondition(conditions: Condition[] | undefined, type: string): Condition | undefined {
  return conditions?.find((c) => c.type === type);
}

export type ResourceStatus = 'Ready' | 'Not Ready' | 'Progressing' | 'Degraded' | 'Rejected' | 'Unknown';

export interface ResourceStatusOptions {
  checkAccepted?: boolean;
}

export function getResourceStatus(
  conditions?: Condition[],
  options?: ResourceStatusOptions,
): ResourceStatus {
  const degraded = getCondition(conditions, 'Degraded');
  if (degraded?.status === 'True') {
    return 'Degraded';
  }

  const progressing = getCondition(conditions, 'Progressing');
  if (progressing?.status === 'True') {
    return 'Progressing';
  }

  if (options?.checkAccepted) {
    const accepted = getCondition(conditions, 'Accepted');
    if (accepted?.status === 'False') {
      return 'Rejected';
    }
  }

  const ready = getCondition(conditions, 'Ready');
  if (ready?.status === 'True') {
    return 'Ready';
  }
  if (ready?.status === 'False') {
    return 'Not Ready';
  }

  return 'Unknown';
}

/** @deprecated Use getResourceStatus */
export type ReadyStatus = ResourceStatus;

/** @deprecated Use getResourceStatus */
export function getReadyStatus(conditions?: Condition[], options?: ResourceStatusOptions): ResourceStatus {
  return getResourceStatus(conditions, options);
}

export function getReadyCondition(conditions?: Condition[]): Condition | undefined {
  return getCondition(conditions, 'Ready');
}
