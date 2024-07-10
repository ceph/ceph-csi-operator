/*
Copyright 2024.

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

// ReadAffinitySpec capture Ceph CSI read affinity settings
type ReadAffinitySpec struct {
	CrushLocationLabels []string `json:"crushLocationLabels,omitempty"`
}

// CephClusterSpec defines the desired state of CephCluster
type CephClusterSpec struct {
	Monitors             []string         `json:"monitors"`
	ReadAffinity         ReadAffinitySpec `json:"readAffinity,omitempty"`
	RbdMirrorDaemonCount int              `json:"rbdMirrorDaemonCount,omitempty"`
}

// CephClusterStatus defines the observed state of CephCluster
type CephClusterStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CephCluster is the Schema for the cephclusters API
type CephCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CephClusterSpec   `json:"spec,omitempty"`
	Status CephClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CephClusterList contains a list of CephCluster
type CephClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CephCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CephCluster{}, &CephClusterList{})
}
