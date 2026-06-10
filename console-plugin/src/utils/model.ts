import { K8sModel } from '@openshift-console/dynamic-plugin-sdk';

export const EngineModel: K8sModel = {
  apiGroup: 'waf.k8s.coraza.io',
  apiVersion: 'v1alpha1',
  kind: 'Engine',
  plural: 'engines',
  abbr: 'ENG',
  namespaced: true,
  label: 'Engine',
  labelPlural: 'Engines',
};

export const RuleSetModel: K8sModel = {
  apiGroup: 'waf.k8s.coraza.io',
  apiVersion: 'v1alpha1',
  kind: 'RuleSet',
  plural: 'rulesets',
  abbr: 'RS',
  namespaced: true,
  label: 'RuleSet',
  labelPlural: 'RuleSets',
};

export const RuleSourceModel: K8sModel = {
  apiGroup: 'waf.k8s.coraza.io',
  apiVersion: 'v1alpha1',
  kind: 'RuleSource',
  plural: 'rulesources',
  abbr: 'RSRC',
  namespaced: true,
  label: 'RuleSource',
  labelPlural: 'RuleSources',
};

export const RuleDataModel: K8sModel = {
  apiGroup: 'waf.k8s.coraza.io',
  apiVersion: 'v1alpha1',
  kind: 'RuleData',
  plural: 'ruledata',
  abbr: 'RD',
  namespaced: true,
  label: 'RuleData',
  labelPlural: 'RuleData',
};

export const ENGINE_GVK = {
  group: 'waf.k8s.coraza.io',
  version: 'v1alpha1',
  kind: 'Engine',
};

export const RULESET_GVK = {
  group: 'waf.k8s.coraza.io',
  version: 'v1alpha1',
  kind: 'RuleSet',
};

export const RULESOURCE_GVK = {
  group: 'waf.k8s.coraza.io',
  version: 'v1alpha1',
  kind: 'RuleSource',
};

export const RULEDATA_GVK = {
  group: 'waf.k8s.coraza.io',
  version: 'v1alpha1',
  kind: 'RuleData',
};
