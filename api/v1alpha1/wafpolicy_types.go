/*
Copyright 2026 Shane Utt.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// -----------------------------------------------------------------------------
// WAFPolicy - Schema Registration
// -----------------------------------------------------------------------------

func init() {
	SchemeBuilder.Register(&WAFPolicy{}, &WAFPolicyList{})
}

// -----------------------------------------------------------------------------
// WAFPolicy
// -----------------------------------------------------------------------------

// WAFPolicy attaches WAF configuration to a Gateway or HTTPRoute using the
// Gateway API Policy Attachment pattern.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target Kind",type=string,JSONPath=`.spec.targetRef.kind`
// +kubebuilder:printcolumn:name="Target Name",type=string,JSONPath=`.spec.targetRef.name`
// +kubebuilder:printcolumn:name="RuleSet",type=string,JSONPath=`.spec.ruleSet.name`
// +kubebuilder:printcolumn:name="Accepted",type=string,JSONPath=`.status.conditions[?(@.type=="Accepted")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type WAFPolicy struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// Spec defines the desired WAF configuration and its target.
	//
	// +required
	Spec WAFPolicySpec `json:"spec"`

	// Status defines the observed state of WAFPolicy.
	//
	// +optional
	Status WAFPolicyStatus `json:"status,omitzero"`
}

// WAFPolicyList contains a list of WAFPolicy resources.
//
// +kubebuilder:object:root=true
type WAFPolicyList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitzero"`

	// +required
	Items []WAFPolicy `json:"items"`
}

// -----------------------------------------------------------------------------
// WAFPolicy - Spec
// -----------------------------------------------------------------------------

// WAFPolicySpec defines the desired WAF configuration and the target it
// applies to.
type WAFPolicySpec struct {
	// TargetRef identifies the Gateway or HTTPRoute this policy applies to.
	// Only resources in the same namespace as the WAFPolicy are supported.
	//
	// +required
	// +kubebuilder:validation:XValidation:rule="self.group == 'gateway.networking.k8s.io'",message="targetRef.group must be gateway.networking.k8s.io"
	// +kubebuilder:validation:XValidation:rule="self.kind == 'Gateway' || self.kind == 'HTTPRoute'",message="targetRef.kind must be Gateway or HTTPRoute"
	TargetRef gwapiv1alpha2.LocalPolicyTargetReferenceWithSectionName `json:"targetRef"`

	// RuleSet references the RuleSet resource that provides the WAF rules.
	//
	// +required
	// +kubebuilder:validation:XValidation:rule="self.name != ''",message="ruleSet name must not be empty"
	RuleSet WAFPolicyRuleSetRef `json:"ruleSet"`

	// FailurePolicy determines the behavior when the WAF is not ready or
	// encounters errors.
	//
	// - "fail": Block traffic
	// - "allow": Allow traffic through
	//
	// +required
	// +kubebuilder:default=fail
	FailurePolicy FailurePolicy `json:"failurePolicy"`
}

// WAFPolicyRuleSetRef is a reference to a RuleSet resource in the same
// namespace.
type WAFPolicyRuleSetRef struct {
	// Name is the name of the RuleSet resource.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

// -----------------------------------------------------------------------------
// WAFPolicy - Status
// -----------------------------------------------------------------------------

// WAFPolicyStatus defines the observed state of WAFPolicy, following
// Gateway API Policy status conventions.
type WAFPolicyStatus struct {
	// Conditions describe the current state of the WAFPolicy.
	//
	// Condition types:
	// - "Accepted": the policy has been validated and accepted
	// - "Programmed": the policy has been translated into an Engine resource
	//
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}
