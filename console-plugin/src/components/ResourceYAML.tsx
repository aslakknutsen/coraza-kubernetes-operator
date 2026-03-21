import * as React from 'react';
import {
  CodeBlock,
  CodeBlockCode,
} from '@patternfly/react-core';
import { K8sResourceCommon } from '@openshift-console/dynamic-plugin-sdk';

interface ResourceYAMLProps {
  resource: K8sResourceCommon;
}

function toYAML(obj: unknown, indent = 0): string {
  const pad = '  '.repeat(indent);
  if (obj === null || obj === undefined) return 'null';
  if (typeof obj === 'string') return obj.includes('\n') ? `|\n${obj.split('\n').map(l => pad + '  ' + l).join('\n')}` : obj;
  if (typeof obj === 'number' || typeof obj === 'boolean') return String(obj);
  if (Array.isArray(obj)) {
    if (obj.length === 0) return '[]';
    return obj.map((item) => `${pad}- ${toYAML(item, indent + 1).trimStart()}`).join('\n');
  }
  if (typeof obj === 'object') {
    const entries = Object.entries(obj as Record<string, unknown>).filter(
      ([, v]) => v !== undefined && v !== null,
    );
    if (entries.length === 0) return '{}';
    return entries
      .map(([k, v]) => {
        const val = toYAML(v, indent + 1);
        if (typeof v === 'object' && !Array.isArray(v)) {
          return `${pad}${k}:\n${val}`;
        }
        if (Array.isArray(v)) {
          return `${pad}${k}:\n${val}`;
        }
        return `${pad}${k}: ${val}`;
      })
      .join('\n');
  }
  return String(obj);
}

export const ResourceYAML: React.FC<ResourceYAMLProps> = ({ resource }) => {
  const yaml = toYAML(resource);
  return (
    <CodeBlock>
      <CodeBlockCode>{yaml}</CodeBlockCode>
    </CodeBlock>
  );
};
