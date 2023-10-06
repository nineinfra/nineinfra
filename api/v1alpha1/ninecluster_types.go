/*
Copyright 2023 nineinfra.

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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NineClusterType describes the type of the nineclusters
type NineClusterType string

// Different types of nineclusters.
const (
	DataHouse   NineClusterType = "DataHouse"
	LakeHouse   NineClusterType = "lakeHouse"
	DataAndLake NineClusterType = "DataAndLake"
)

const (
	StateAvailable = "Available"
	StateDeploying = "Deploying"
	StateFailed    = "Failed"
)

// NineClusterSpec defines the desired state of NineCluster
type NineClusterSpec struct {
	// Type of the ninecluster. default value DataHouse.
	Type NineClusterType `json:"type,omitempty"`
	// Data Volume of the ninecluster. The unit of the data volume is Gi.
	DataVolume int32 `json:"dataVolume"`
}

// NineClusterStatus defines the observed state of NineCluster
type NineClusterStatus struct {
	// Represents the observations of a NineCluster's current state.
	// NineCluster.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// NineCluster.status.conditions.status are one of True, False, Unknown.
	// NineCluster.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// NineCluster.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NineCluster is the Schema for the nineclusters API
type NineCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NineClusterSpec   `json:"spec,omitempty"`
	Status NineClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NineClusterList contains a list of NineCluster
type NineClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NineCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NineCluster{}, &NineClusterList{})
}
