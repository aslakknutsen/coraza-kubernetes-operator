/*
Copyright Coraza Kubernetes Operator contributors.

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

const (
	// ConditionTypeReady indicates the resource is ready for use.
	ConditionTypeReady = "Ready"

	// ConditionTypeAccepted indicates the Engine target is valid and not contested.
	ConditionTypeAccepted = "Accepted"

	// ConditionReasonPending is the reason for CRD-default unreconciled conditions.
	ConditionReasonPending = "Pending"

	// InitialConditionMessage is the message for CRD-default unreconciled conditions.
	InitialConditionMessage = "Waiting for controller"

	// InitialConditionLastTransitionTime is the placeholder timestamp for CRD-default
	// unreconciled conditions before the controller has observed the resource.
	InitialConditionLastTransitionTime = "1970-01-01T00:00:00Z"
)
