import { K8sResourceCommon } from '@openshift-console/dynamic-plugin-sdk';

export interface Condition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
  lastTransitionTime?: string;
}

// ---------------------------------------------------------------------------
// Engine
// ---------------------------------------------------------------------------

export interface EngineTarget {
  type: string;
  name?: string;
}

export interface WasmDriverConfig {
  image?: string;
  imagePullSecret?: string;
}

export interface DriverConfig {
  type?: string;
  wasm?: WasmDriverConfig;
}

export interface RuleSetCacheServerConfig {
  pollIntervalSeconds?: number;
}

export interface RuleSetReference {
  name: string;
}

export interface EngineSpec {
  ruleSet?: RuleSetReference;
  target?: EngineTarget;
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

// ---------------------------------------------------------------------------
// RuleSet
// ---------------------------------------------------------------------------

export interface SourceReference {
  name: string;
}

export interface DataReference {
  name: string;
}

export interface RuleSetSpec {
  sources?: SourceReference[];
  data?: DataReference[];
}

export interface RuleSetStatus {
  conditions?: Condition[];
}

export interface RuleSetResource extends K8sResourceCommon {
  spec?: RuleSetSpec;
  status?: RuleSetStatus;
}

// ---------------------------------------------------------------------------
// RuleSource
// ---------------------------------------------------------------------------

export interface RuleSourceSpec {
  rules: string;
}

export interface RuleSourceResource extends K8sResourceCommon {
  spec: RuleSourceSpec;
}

// ---------------------------------------------------------------------------
// RuleData
// ---------------------------------------------------------------------------

export interface RuleDataSpec {
  files: Record<string, string>;
}

export interface RuleDataResource extends K8sResourceCommon {
  spec: RuleDataSpec;
}
