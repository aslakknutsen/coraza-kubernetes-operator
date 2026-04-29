import { describe, expect, it } from 'vitest';
import { getReadyCondition, getReadyStatus } from './status';
import type { Condition } from './types';

describe('getReadyCondition', () => {
  it('returns undefined when conditions missing', () => {
    expect(getReadyCondition(undefined)).toBeUndefined();
    expect(getReadyCondition([])).toBeUndefined();
  });

  it('returns the Ready row when multiple conditions exist', () => {
    const conditions: Condition[] = [
      { type: 'Synced', status: 'True' },
      { type: 'Ready', status: 'False', reason: 'ValidationFailed' },
    ];
    expect(getReadyCondition(conditions)?.type).toBe('Ready');
  });
});

describe('getReadyStatus', () => {
  it('returns Unknown when there is no Ready condition', () => {
    expect(getReadyStatus(undefined)).toBe('Unknown');
    expect(getReadyStatus([])).toBe('Unknown');
    expect(getReadyStatus([{ type: 'Synced', status: 'True' }])).toBe('Unknown');
  });

  it('returns Ready when Ready status is True', () => {
    expect(
      getReadyStatus([{ type: 'Ready', status: 'True' }]),
    ).toBe('Ready');
  });

  it('returns Progressing when Ready is False with Progressing reason', () => {
    expect(
      getReadyStatus([{ type: 'Ready', status: 'False', reason: 'Progressing' }]),
    ).toBe('Progressing');
  });

  it('returns Not Ready when Ready is False without progressing reason', () => {
    expect(
      getReadyStatus([{ type: 'Ready', status: 'False', reason: 'Degraded' }]),
    ).toBe('Not Ready');
  });

  it('returns Unknown for unexpected Ready status string', () => {
    expect(
      getReadyStatus([{ type: 'Ready', status: 'Unknown' }]),
    ).toBe('Unknown');
  });
});
