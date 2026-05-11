import { K8sModel } from '@openshift-console/dynamic-plugin-sdk';

const API_GROUP = 'waf.k8s.coraza.io';
const API_VERSION = 'v1alpha1';

export const EngineModel: K8sModel = {
  apiGroup: API_GROUP,
  apiVersion: API_VERSION,
  kind: 'Engine',
  plural: 'engines',
  abbr: 'ENG',
  namespaced: true,
  label: 'Engine',
  labelPlural: 'Engines',
};

export const RuleSetModel: K8sModel = {
  apiGroup: API_GROUP,
  apiVersion: API_VERSION,
  kind: 'RuleSet',
  plural: 'rulesets',
  abbr: 'RS',
  namespaced: true,
  label: 'RuleSet',
  labelPlural: 'RuleSets',
};

export const RuleSourceModel: K8sModel = {
  apiGroup: API_GROUP,
  apiVersion: API_VERSION,
  kind: 'RuleSource',
  plural: 'rulesources',
  abbr: 'RSRC',
  namespaced: true,
  label: 'RuleSource',
  labelPlural: 'RuleSources',
};

export const RuleDataModel: K8sModel = {
  apiGroup: API_GROUP,
  apiVersion: API_VERSION,
  kind: 'RuleData',
  plural: 'ruledata',
  abbr: 'RD',
  namespaced: true,
  label: 'RuleData',
  labelPlural: 'RuleData',
};

export const ENGINE_GVK = {
  group: API_GROUP,
  version: API_VERSION,
  kind: 'Engine',
};

export const RULESET_GVK = {
  group: API_GROUP,
  version: API_VERSION,
  kind: 'RuleSet',
};

export const RULESOURCE_GVK = {
  group: API_GROUP,
  version: API_VERSION,
  kind: 'RuleSource',
};

export const RULEDATA_GVK = {
  group: API_GROUP,
  version: API_VERSION,
  kind: 'RuleData',
};
