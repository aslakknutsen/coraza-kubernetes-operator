import { K8sResourceCommon } from '@openshift-console/dynamic-plugin-sdk';

export interface RuleSourceReference {
  name: string;
}

export interface RuleSetSpec {
  rules: RuleSourceReference[];
  ruleData?: string;
}

export interface RuleSetStatus {
  conditions?: Condition[];
}

export interface RuleSetResource extends K8sResourceCommon {
  spec: RuleSetSpec;
  status?: RuleSetStatus;
}

export interface RuleSetCacheServerConfig {
  pollIntervalSeconds: number;
}

export interface IstioWasmConfig {
  mode: string;
  image: string;
  workloadSelector?: {
    matchLabels?: Record<string, string>;
  };
  ruleSetCacheServer?: RuleSetCacheServerConfig;
}

export interface IstioDriverConfig {
  wasm?: IstioWasmConfig;
}

export interface DriverConfig {
  istio?: IstioDriverConfig;
}

export interface EngineSpec {
  ruleSet: { name: string };
  driver: DriverConfig;
  failurePolicy: string;
}

export interface EngineStatus {
  conditions?: Condition[];
  gateways?: { name: string }[];
}

export interface EngineResource extends K8sResourceCommon {
  spec: EngineSpec;
  status?: EngineStatus;
}

export interface Condition {
  type: string;
  status: string;
  reason?: string;
  message?: string;
  lastTransitionTime?: string;
}
