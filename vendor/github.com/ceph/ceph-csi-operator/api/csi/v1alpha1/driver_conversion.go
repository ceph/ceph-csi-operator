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
	"errors"
	"log"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	csiv1beta1 "github.com/ceph/ceph-csi-operator/api/csi/v1beta1"
)

func convertVolumeSpec[Src, Dest any](elems []Src, convertor func(Src) Dest) []Dest {
	if elems == nil {
		return nil
	}

	dst := make([]Dest, len(elems))
	for i, v := range elems {
		dst[i] = convertor(v)
	}

	return dst
}

// ConvertTo converts this Driver (v1alpha1) to the Hub version (v1beta1).
func (src *Driver) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*csiv1beta1.Driver)
	if !ok {
		return errors.New("convertto: failed to cast to v1beta1 Driver")
	}

	log.Printf("ConvertTo: Converting Driver from Spoke version v1alpha1 to Hub version v1beta1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.AttachRequired = src.Spec.AttachRequired
	dst.Spec.CephFsClientType = csiv1beta1.CephFsClientType(src.Spec.CephFsClientType)
	dst.Spec.ClusterName = src.Spec.ClusterName
	dst.Spec.ControllerPlugin = &csiv1beta1.ControllerPluginSpec{
		HostNetwork: src.Spec.ControllerPlugin.HostNetwork,
		PodCommonSpec: csiv1beta1.PodCommonSpec{
			ServiceAccountName: src.Spec.NodePlugin.PodCommonSpec.ServiceAccountName,
			PrioritylClassName: src.Spec.NodePlugin.PodCommonSpec.PrioritylClassName,
			Labels:             src.Spec.NodePlugin.PodCommonSpec.Labels,
			Annotations:        src.Spec.NodePlugin.PodCommonSpec.Annotations,
			Affinity:           src.Spec.NodePlugin.PodCommonSpec.Affinity,
			Tolerations:        src.Spec.NodePlugin.PodCommonSpec.Tolerations,
			Volumes: convertVolumeSpec(src.Spec.NodePlugin.PodCommonSpec.Volumes, func(v VolumeSpec) csiv1beta1.VolumeSpec {
				return csiv1beta1.VolumeSpec{
					Volume: v.Volume,
					Mount:  v.Mount,
				}
			}),
			ImagePullPolicy: src.Spec.NodePlugin.PodCommonSpec.ImagePullPolicy,
		},
		DeploymentStrategy: src.Spec.ControllerPlugin.DeploymentStrategy,
		Replicas:           src.Spec.ControllerPlugin.Replicas,
		Resources:          csiv1beta1.ControllerPluginResourcesSpec(src.Spec.ControllerPlugin.Resources),
		Privileged:         src.Spec.ControllerPlugin.Privileged,
	}
	dst.Spec.DeployCsiAddons = src.Spec.DeployCsiAddons
	dst.Spec.EnableMetadata = src.Spec.EnableMetadata
	dst.Spec.Encryption = (*csiv1beta1.EncryptionSpec)(src.Spec.Encryption)
	dst.Spec.FsGroupPolicy = src.Spec.FsGroupPolicy
	dst.Spec.FuseMountOptions = src.Spec.FuseMountOptions
	dst.Spec.GRpcTimeout = src.Spec.GRpcTimeout
	dst.Spec.GenerateOMapInfo = src.Spec.GenerateOMapInfo
	dst.Spec.ImageSet = src.Spec.ImageSet
	dst.Spec.KernelMountOptions = src.Spec.KernelMountOptions
	dst.Spec.LeaderElection = (*csiv1beta1.LeaderElectionSpec)(src.Spec.LeaderElection)
	dst.Spec.Liveness = (*csiv1beta1.LivenessSpec)(src.Spec.Liveness)
	dst.Spec.Log = &csiv1beta1.LogSpec{
		Verbosity: src.Spec.Log.Verbosity,
		Rotation: &csiv1beta1.LogRotationSpec{
			MaxFiles:    src.Spec.Log.Rotation.MaxFiles,
			MaxLogSize:  src.Spec.Log.Rotation.MaxLogSize,
			Periodicity: csiv1beta1.PeriodicityType(src.Spec.Log.Rotation.Periodicity),
			LogHostPath: src.Spec.Log.Rotation.LogHostPath,
		},
	}
	dst.Spec.NodePlugin = &csiv1beta1.NodePluginSpec{
		PodCommonSpec: csiv1beta1.PodCommonSpec{
			ServiceAccountName: src.Spec.NodePlugin.PodCommonSpec.ServiceAccountName,
			PrioritylClassName: src.Spec.NodePlugin.PodCommonSpec.PrioritylClassName,
			Labels:             src.Spec.NodePlugin.PodCommonSpec.Labels,
			Annotations:        src.Spec.NodePlugin.PodCommonSpec.Annotations,
			Affinity:           src.Spec.NodePlugin.PodCommonSpec.Affinity,
			Tolerations:        src.Spec.NodePlugin.PodCommonSpec.Tolerations,
			Volumes: convertVolumeSpec(src.Spec.NodePlugin.PodCommonSpec.Volumes, func(v VolumeSpec) csiv1beta1.VolumeSpec {
				return csiv1beta1.VolumeSpec{
					Volume: v.Volume,
					Mount:  v.Mount,
				}
			}),
			ImagePullPolicy: src.Spec.NodePlugin.PodCommonSpec.ImagePullPolicy,
		},
	}
	dst.Spec.SnapshotPolicy = csiv1beta1.SnapshotPolicyType(src.Spec.SnapshotPolicy)

	return nil
}

// ConvertFrom converts the Hub version (v1beta1) to this Driver (v1alpha1).
func (dst *Driver) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*csiv1beta1.Driver)
	if !ok {
		return errors.New("convertfrom: failed to cast to v1beta1 Driver")
	}

	log.Printf("ConvertFrom: Converting Driver from Hub version v1beta1 to Spoke version v1alpha1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.AttachRequired = src.Spec.AttachRequired
	dst.Spec.CephFsClientType = CephFsClientType(src.Spec.CephFsClientType)
	dst.Spec.ClusterName = src.Spec.ClusterName
	dst.Spec.ControllerPlugin = &ControllerPluginSpec{
		HostNetwork: src.Spec.ControllerPlugin.HostNetwork,
		PodCommonSpec: PodCommonSpec{
			ServiceAccountName: src.Spec.NodePlugin.PodCommonSpec.ServiceAccountName,
			PrioritylClassName: src.Spec.NodePlugin.PodCommonSpec.PrioritylClassName,
			Labels:             src.Spec.NodePlugin.PodCommonSpec.Labels,
			Annotations:        src.Spec.NodePlugin.PodCommonSpec.Annotations,
			Affinity:           src.Spec.NodePlugin.PodCommonSpec.Affinity,
			Tolerations:        src.Spec.NodePlugin.PodCommonSpec.Tolerations,
			Volumes: convertVolumeSpec(src.Spec.NodePlugin.PodCommonSpec.Volumes, func(v csiv1beta1.VolumeSpec) VolumeSpec {
				return VolumeSpec{
					Volume: v.Volume,
					Mount:  v.Mount,
				}
			}),
			ImagePullPolicy: src.Spec.NodePlugin.PodCommonSpec.ImagePullPolicy,
		},
		DeploymentStrategy: src.Spec.ControllerPlugin.DeploymentStrategy,
		Replicas:           src.Spec.ControllerPlugin.Replicas,
		Resources:          ControllerPluginResourcesSpec(src.Spec.ControllerPlugin.Resources),
		Privileged:         src.Spec.ControllerPlugin.Privileged,
	}
	dst.Spec.DeployCsiAddons = src.Spec.DeployCsiAddons
	dst.Spec.EnableMetadata = src.Spec.EnableMetadata
	dst.Spec.Encryption = (*EncryptionSpec)(src.Spec.Encryption)
	dst.Spec.FsGroupPolicy = src.Spec.FsGroupPolicy
	dst.Spec.FuseMountOptions = src.Spec.FuseMountOptions
	dst.Spec.GRpcTimeout = src.Spec.GRpcTimeout
	dst.Spec.GenerateOMapInfo = src.Spec.GenerateOMapInfo
	dst.Spec.ImageSet = src.Spec.ImageSet
	dst.Spec.KernelMountOptions = src.Spec.KernelMountOptions
	dst.Spec.LeaderElection = (*LeaderElectionSpec)(src.Spec.LeaderElection)
	dst.Spec.Liveness = (*LivenessSpec)(src.Spec.Liveness)
	dst.Spec.Log = &LogSpec{
		Verbosity: src.Spec.Log.Verbosity,
		Rotation: &LogRotationSpec{
			MaxFiles:    src.Spec.Log.Rotation.MaxFiles,
			MaxLogSize:  src.Spec.Log.Rotation.MaxLogSize,
			Periodicity: PeriodicityType(src.Spec.Log.Rotation.Periodicity),
			LogHostPath: src.Spec.Log.Rotation.LogHostPath,
		},
	}
	dst.Spec.NodePlugin = &NodePluginSpec{
		PodCommonSpec: PodCommonSpec{
			ServiceAccountName: src.Spec.NodePlugin.PodCommonSpec.ServiceAccountName,
			PrioritylClassName: src.Spec.NodePlugin.PodCommonSpec.PrioritylClassName,
			Labels:             src.Spec.NodePlugin.PodCommonSpec.Labels,
			Annotations:        src.Spec.NodePlugin.PodCommonSpec.Annotations,
			Affinity:           src.Spec.NodePlugin.PodCommonSpec.Affinity,
			Tolerations:        src.Spec.NodePlugin.PodCommonSpec.Tolerations,
			Volumes: convertVolumeSpec(src.Spec.NodePlugin.PodCommonSpec.Volumes, func(v csiv1beta1.VolumeSpec) VolumeSpec {
				return VolumeSpec{
					Volume: v.Volume,
					Mount:  v.Mount,
				}
			}),
			ImagePullPolicy: src.Spec.NodePlugin.PodCommonSpec.ImagePullPolicy,
		},
	}
	dst.Spec.SnapshotPolicy = SnapshotPolicyType(src.Spec.SnapshotPolicy)

	return nil
}
