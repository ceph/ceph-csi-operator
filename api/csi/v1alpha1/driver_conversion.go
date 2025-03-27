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

func ConvertAlphaDriverToBeta(spec DriverSpec) csiv1beta1.DriverSpec {
	ret := csiv1beta1.DriverSpec{}

	ret.AttachRequired = spec.AttachRequired
	ret.CephFsClientType = csiv1beta1.CephFsClientType(spec.CephFsClientType)
	ret.ClusterName = spec.ClusterName
	if spec.ControllerPlugin != nil {
		ret.ControllerPlugin = &csiv1beta1.ControllerPluginSpec{
			HostNetwork: spec.ControllerPlugin.HostNetwork,
			PodCommonSpec: csiv1beta1.PodCommonSpec{
				ServiceAccountName: spec.ControllerPlugin.PodCommonSpec.ServiceAccountName,
				PrioritylClassName: spec.ControllerPlugin.PodCommonSpec.PrioritylClassName,
				Labels:             spec.ControllerPlugin.PodCommonSpec.Labels,
				Annotations:        spec.ControllerPlugin.PodCommonSpec.Annotations,
				Affinity:           spec.ControllerPlugin.PodCommonSpec.Affinity,
				Tolerations:        spec.ControllerPlugin.PodCommonSpec.Tolerations,
				Volumes: convertVolumeSpec(spec.ControllerPlugin.PodCommonSpec.Volumes, func(v VolumeSpec) csiv1beta1.VolumeSpec {
					return csiv1beta1.VolumeSpec{
						Volume: v.Volume,
						Mount:  v.Mount,
					}
				}),
				ImagePullPolicy: spec.ControllerPlugin.PodCommonSpec.ImagePullPolicy,
			},
			DeploymentStrategy: spec.ControllerPlugin.DeploymentStrategy,
			Replicas:           spec.ControllerPlugin.Replicas,
			Resources:          csiv1beta1.ControllerPluginResourcesSpec(spec.ControllerPlugin.Resources),
			Privileged:         spec.ControllerPlugin.Privileged,
		}
	}
	ret.DeployCsiAddons = spec.DeployCsiAddons
	ret.EnableMetadata = spec.EnableMetadata
	if spec.Encryption != nil {
		ret.Encryption = (*csiv1beta1.EncryptionSpec)(spec.Encryption)
	}
	ret.FsGroupPolicy = spec.FsGroupPolicy
	ret.FuseMountOptions = spec.FuseMountOptions
	ret.GRpcTimeout = spec.GRpcTimeout
	ret.GenerateOMapInfo = spec.GenerateOMapInfo
	ret.ImageSet = spec.ImageSet
	ret.KernelMountOptions = spec.KernelMountOptions
	if spec.LeaderElection != nil {
		ret.LeaderElection = (*csiv1beta1.LeaderElectionSpec)(spec.LeaderElection)
	}
	if spec.Liveness != nil {
		ret.Liveness = (*csiv1beta1.LivenessSpec)(spec.Liveness)
	}
	if spec.Log != nil {
		ret.Log = &csiv1beta1.LogSpec{
			Verbosity: spec.Log.Verbosity,
		}
		if spec.Log.Rotation != nil {
			ret.Log.Rotation = &csiv1beta1.LogRotationSpec{
				MaxFiles:    spec.Log.Rotation.MaxFiles,
				MaxLogSize:  spec.Log.Rotation.MaxLogSize,
				Periodicity: csiv1beta1.PeriodicityType(spec.Log.Rotation.Periodicity),
				LogHostPath: spec.Log.Rotation.LogHostPath,
			}
		}
	}
	if spec.NodePlugin != nil {
		ret.NodePlugin = &csiv1beta1.NodePluginSpec{
			PodCommonSpec: csiv1beta1.PodCommonSpec{
				ServiceAccountName: spec.NodePlugin.PodCommonSpec.ServiceAccountName,
				PrioritylClassName: spec.NodePlugin.PodCommonSpec.PrioritylClassName,
				Labels:             spec.NodePlugin.PodCommonSpec.Labels,
				Annotations:        spec.NodePlugin.PodCommonSpec.Annotations,
				Affinity:           spec.NodePlugin.PodCommonSpec.Affinity,
				Tolerations:        spec.NodePlugin.PodCommonSpec.Tolerations,
				Volumes: convertVolumeSpec(spec.NodePlugin.PodCommonSpec.Volumes, func(v VolumeSpec) csiv1beta1.VolumeSpec {
					return csiv1beta1.VolumeSpec{
						Volume: v.Volume,
						Mount:  v.Mount,
					}
				}),
				ImagePullPolicy: spec.NodePlugin.PodCommonSpec.ImagePullPolicy,
			},
		}
	}
	ret.SnapshotPolicy = csiv1beta1.SnapshotPolicyType(spec.SnapshotPolicy)

	return ret
}

func ConvertBetaDriverToAlpha(spec csiv1beta1.DriverSpec) DriverSpec {
	ret := DriverSpec{}

	ret.AttachRequired = spec.AttachRequired
	ret.CephFsClientType = CephFsClientType(spec.CephFsClientType)
	ret.ClusterName = spec.ClusterName
	if spec.ControllerPlugin != nil {
		ret.ControllerPlugin = &ControllerPluginSpec{
			HostNetwork: spec.ControllerPlugin.HostNetwork,
			PodCommonSpec: PodCommonSpec{
				ServiceAccountName: spec.NodePlugin.PodCommonSpec.ServiceAccountName,
				PrioritylClassName: spec.NodePlugin.PodCommonSpec.PrioritylClassName,
				Labels:             spec.NodePlugin.PodCommonSpec.Labels,
				Annotations:        spec.NodePlugin.PodCommonSpec.Annotations,
				Affinity:           spec.NodePlugin.PodCommonSpec.Affinity,
				Tolerations:        spec.NodePlugin.PodCommonSpec.Tolerations,
				Volumes: convertVolumeSpec(spec.NodePlugin.PodCommonSpec.Volumes, func(v csiv1beta1.VolumeSpec) VolumeSpec {
					return VolumeSpec{
						Volume: v.Volume,
						Mount:  v.Mount,
					}
				}),
				ImagePullPolicy: spec.NodePlugin.PodCommonSpec.ImagePullPolicy,
			},
			DeploymentStrategy: spec.ControllerPlugin.DeploymentStrategy,
			Replicas:           spec.ControllerPlugin.Replicas,
			Resources:          ControllerPluginResourcesSpec(spec.ControllerPlugin.Resources),
			Privileged:         spec.ControllerPlugin.Privileged,
		}
	}
	ret.DeployCsiAddons = spec.DeployCsiAddons
	ret.EnableMetadata = spec.EnableMetadata
	if spec.Encryption != nil {
		ret.Encryption = (*EncryptionSpec)(spec.Encryption)
	}
	ret.FsGroupPolicy = spec.FsGroupPolicy
	ret.FuseMountOptions = spec.FuseMountOptions
	ret.GRpcTimeout = spec.GRpcTimeout
	ret.GenerateOMapInfo = spec.GenerateOMapInfo
	ret.ImageSet = spec.ImageSet
	ret.KernelMountOptions = spec.KernelMountOptions
	if spec.LeaderElection != nil {
		ret.LeaderElection = (*LeaderElectionSpec)(spec.LeaderElection)
	}
	if spec.Liveness != nil {
		ret.Liveness = (*LivenessSpec)(spec.Liveness)
	}
	if spec.Log != nil {
		ret.Log = &LogSpec{
			Verbosity: spec.Log.Verbosity,
		}
		if spec.Log.Rotation != nil {
			ret.Log.Rotation = &LogRotationSpec{
				MaxFiles:    spec.Log.Rotation.MaxFiles,
				MaxLogSize:  spec.Log.Rotation.MaxLogSize,
				Periodicity: PeriodicityType(spec.Log.Rotation.Periodicity),
				LogHostPath: spec.Log.Rotation.LogHostPath,
			}
		}
	}
	if spec.NodePlugin != nil {
		ret.NodePlugin = &NodePluginSpec{
			PodCommonSpec: PodCommonSpec{
				ServiceAccountName: spec.NodePlugin.PodCommonSpec.ServiceAccountName,
				PrioritylClassName: spec.NodePlugin.PodCommonSpec.PrioritylClassName,
				Labels:             spec.NodePlugin.PodCommonSpec.Labels,
				Annotations:        spec.NodePlugin.PodCommonSpec.Annotations,
				Affinity:           spec.NodePlugin.PodCommonSpec.Affinity,
				Tolerations:        spec.NodePlugin.PodCommonSpec.Tolerations,
				Volumes: convertVolumeSpec(spec.NodePlugin.PodCommonSpec.Volumes, func(v csiv1beta1.VolumeSpec) VolumeSpec {
					return VolumeSpec{
						Volume: v.Volume,
						Mount:  v.Mount,
					}
				}),
				ImagePullPolicy: spec.NodePlugin.PodCommonSpec.ImagePullPolicy,
			},
		}
	}
	ret.SnapshotPolicy = SnapshotPolicyType(spec.SnapshotPolicy)

	return ret
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
	dst.Spec = ConvertAlphaDriverToBeta(src.Spec)

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
	dst.Spec = ConvertBetaDriverToAlpha(src.Spec)

	return nil
}
