import { K8sResourceCommon } from '@openshift-console/dynamic-plugin-sdk';

export interface Condition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
  lastTransitionTime?: string;
  observedGeneration?: number;
}

export interface SourceReference {
  name: string;
}

export interface DataReference {
  name: string;
}

export interface RuleSetSpec {
  sources: SourceReference[];
  data?: DataReference[];
}

export interface RuleSetStatus {
  conditions?: Condition[];
}

export interface RuleSetResource extends K8sResourceCommon {
  spec: RuleSetSpec;
  status?: RuleSetStatus;
}

export interface RuleSetCacheServerConfig {
  pollIntervalSeconds?: number;
}

export interface EngineTarget {
  type: string;
  name: string;
  provider?: string;
}

export interface WasmDriverConfig {
  image?: string;
  imagePullSecret?: string;
}

export interface DriverConfig {
  type?: string;
  wasm?: WasmDriverConfig;
}

export interface EngineSpec {
  ruleSet: { name: string };
  target: EngineTarget;
  failurePolicy?: string;
  ruleSetCacheServer?: RuleSetCacheServerConfig;
  driver?: DriverConfig;
}

export interface EngineStatus {
  conditions?: Condition[];
}

export interface EngineResource extends K8sResourceCommon {
  spec?: EngineSpec;
  status?: EngineStatus;
}

export interface RuleSourceSpec {
  rules: string;
}

export interface RuleSourceStatus {
  conditions?: Condition[];
}

export interface RuleSourceResource extends K8sResourceCommon {
  spec: RuleSourceSpec;
  status?: RuleSourceStatus;
}

export interface RuleDataSpec {
  files: Record<string, string>;
}

export interface RuleDataStatus {
  conditions?: Condition[];
}

export interface RuleDataResource extends K8sResourceCommon {
  spec: RuleDataSpec;
  status?: RuleDataStatus;
}
