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

package utils

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	SocketDir         = "/csi"
	csiEndpoint       = "unix://" + SocketDir + "/csi.sock"
	csiAddonsEndpoint = "unix://" + SocketDir + "/csi-addons.sock"

	kmsConfigVolumeName      = "ceph-csi-kms-config"
	registrationVolumeName   = "registration-dir"
	pluginDirVolumeName      = "plugin-dir"
	podsMountDirVolumeName   = "pods-mount-dir"
	pluginMountDirVolumeName = "plugin-mount-dir"

	CsiConfigMapConfigKey  = "config.json"
	CsiConfigMapMappingKey = "cluster-mapping.json"

	logsDirVolumeName      = "logs-dir"
	logRotateDirVolumeName = "log-rotate-dir"
)

// Ceph CSI common volumes
var SocketDirVolume = corev1.Volume{
	Name: "socket-dir",
	VolumeSource: corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{
			Medium: corev1.StorageMediumMemory,
		},
	},
}
var HostDevVolume = corev1.Volume{
	Name: "host-dev",
	VolumeSource: corev1.VolumeSource{
		HostPath: &corev1.HostPathVolumeSource{
			Path: "/dev",
		},
	},
}
var HostSysVolume = corev1.Volume{
	Name: "host-sys",
	VolumeSource: corev1.VolumeSource{
		HostPath: &corev1.HostPathVolumeSource{
			Path: "/sys",
		},
	},
}
var HostRunMountVolume = corev1.Volume{
	Name: "host-run-mount",
	VolumeSource: corev1.VolumeSource{
		HostPath: &corev1.HostPathVolumeSource{
			Path: "/run/mount",
		},
	},
}
var LibModulesVolume = corev1.Volume{
	Name: "lib-modules",
	VolumeSource: corev1.VolumeSource{
		HostPath: &corev1.HostPathVolumeSource{
			Path: "/lib/modules",
		},
	},
}
var KeysTmpDirVolume = corev1.Volume{
	Name: "keys-tmp-dir",
	VolumeSource: corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{
			Medium: corev1.StorageMediumMemory,
		},
	},
}
var OidcTokenVolume = corev1.Volume{
	Name: "oidc-token",
	VolumeSource: corev1.VolumeSource{
		Projected: &corev1.ProjectedVolumeSource{
			Sources: []corev1.VolumeProjection{
				{
					ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
						Path:              "oidc-token",
						ExpirationSeconds: ptr.To(int64(3600)),
						Audience:          "ceph-csi-kms",
					},
				},
			},
		},
	},
}
var CsiConfigVolume = corev1.Volume{
	Name: "ceph-csi-config",
	VolumeSource: corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "ceph-csi-config",
			},
		},
	},
}
var EtcSelinuxVolume = corev1.Volume{
	Name: "etc-selinux",
	VolumeSource: corev1.VolumeSource{
		HostPath: &corev1.HostPathVolumeSource{
			Path: "/etc/selinux",
		},
	},
}

func KmsConfigVolume(configRef *corev1.LocalObjectReference) corev1.Volume {
	return corev1.Volume{
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: *configRef,
				Items: []corev1.KeyToPath{
					{
						Key:  "config.json",
						Path: "config.json",
					},
				},
			},
		},
	}
}
func PluginMountDirVolume(kubeletDirPath string) corev1.Volume {
	return corev1.Volume{
		Name: pluginMountDirVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: fmt.Sprintf("%s/plugins", kubeletDirPath),
				Type: ptr.To(corev1.HostPathDirectory),
			},
		},
	}
}
func PluginDirVolume(kubeletDirPath string, driverNamePrefix string) corev1.Volume {
	return corev1.Volume{
		Name: pluginDirVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: fmt.Sprintf("%s/plugins/%s", kubeletDirPath, driverNamePrefix),
				Type: ptr.To(corev1.HostPathDirectoryOrCreate),
			},
		},
	}
}
func RegistrationDirVolume(kubeletDirPath string) corev1.Volume {
	return corev1.Volume{
		Name: registrationVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: fmt.Sprintf("%s/plugins_registry", kubeletDirPath),
				Type: ptr.To(corev1.HostPathDirectory),
			},
		},
	}
}
func PodsMountDirVolume(kubeletDirPath string) corev1.Volume {
	return corev1.Volume{
		Name: podsMountDirVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: fmt.Sprintf("%s/pods", kubeletDirPath),
				Type: ptr.To(corev1.HostPathDirectory),
			},
		},
	}
}
func LogsDirVolume(logHostPath, pluginName string) corev1.Volume {
	return corev1.Volume{
		Name: logsDirVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: fmt.Sprintf("%s/%s", logHostPath, pluginName),
				Type: ptr.To(corev1.HostPathDirectoryOrCreate),
			},
		},
	}
}
func LogRotateDirVolumeName(driverName string) corev1.Volume {
	return corev1.Volume{
		Name: logRotateDirVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: LogRotateConfigMapName(driverName),
				},
			},
		},
	}
}

// Ceph CSI common volume Mounts
var SocketDirVolumeMount = corev1.VolumeMount{
	Name:      SocketDirVolume.Name,
	MountPath: SocketDir,
}
var HostDevVolumeMount = corev1.VolumeMount{
	Name:      HostDevVolume.Name,
	MountPath: "/dev",
}
var HostSysVolumeMount = corev1.VolumeMount{
	Name:      HostSysVolume.Name,
	MountPath: "/sys",
}
var HostRunMountVolumeMount = corev1.VolumeMount{
	Name:      HostRunMountVolume.Name,
	MountPath: "/run/mount",
}
var LibModulesVolumeMount = corev1.VolumeMount{
	Name:      LibModulesVolume.Name,
	MountPath: "/lib/modules",
	ReadOnly:  true,
}
var KeysTmpDirVolumeMount = corev1.VolumeMount{
	Name:      KeysTmpDirVolume.Name,
	MountPath: "/tmp/csi/keys",
}
var OidcTokenVolumeMount = corev1.VolumeMount{
	Name:      OidcTokenVolume.Name,
	MountPath: "/run/secrets/tokens",
	ReadOnly:  true,
}
var CsiConfigVolumeMount = corev1.VolumeMount{
	Name:      CsiConfigVolume.Name,
	MountPath: "/etc/ceph-csi-config",
}
var KmsConfigVolumeMount = corev1.VolumeMount{
	Name:      kmsConfigVolumeName,
	MountPath: "/etc/ceph-csi-encryption-kms-config/",
	ReadOnly:  true,
}
var PluginDirVolumeMount = corev1.VolumeMount{
	Name:      pluginDirVolumeName,
	MountPath: "/csi",
}
var RegistrationDirVolumeMount = corev1.VolumeMount{
	Name:      registrationVolumeName,
	MountPath: "/registration",
}
var EtcSelinuxVolumeMount = corev1.VolumeMount{
	Name:      EtcSelinuxVolume.Name,
	MountPath: "/etc/selinux",
	ReadOnly:  true,
}
var LogsDirVolumeMount = corev1.VolumeMount{
	Name:      logsDirVolumeName,
	MountPath: "/csi-logs",
}
var LogRotateDirVolumeMount = corev1.VolumeMount{
	Name:      logRotateDirVolumeName,
	MountPath: "/logrotate-config",
}

func PodsMountDirVolumeMount(kubletDirPath string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:             podsMountDirVolumeName,
		MountPath:        fmt.Sprintf("%s/pods", kubletDirPath),
		MountPropagation: ptr.To(corev1.MountPropagationBidirectional),
	}
}
func PluginMountDirVolumeMount(kubletDirPath string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:             pluginMountDirVolumeName,
		MountPath:        fmt.Sprintf("%s/plugins", kubletDirPath),
		MountPropagation: ptr.To(corev1.MountPropagationBidirectional),
	}
}

// Ceph CSI Common env var definition
var PodIpEnvVar = corev1.EnvVar{
	Name: "POD_IP",
	ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "status.podIP",
		},
	},
}
var PodNameEnvVar = corev1.EnvVar{
	Name: "POD_NAME",
	ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "metadata.name",
		},
	},
}
var PodNamespaceEnvVar = corev1.EnvVar{
	Name: "POD_NAMESPACE",
	ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "metadata.namespace",
		},
	},
}
var PodUidEnvVar = corev1.EnvVar{
	Name: "POD_UID",
	ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "metadata.uid",
		},
	},
}
var NodeIdEnvVar = corev1.EnvVar{
	Name: "NODE_ID",
	ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "spec.nodeName",
		},
	},
}
var DriverNamespaceEnvVar = corev1.EnvVar{
	Name: "DRIVER_NAMESPACE",
	ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "metadata.namespace",
		},
	},
}

// CSI Addons container port definition
var CsiAddonsContainerPort = corev1.ContainerPort{
	ContainerPort: int32(9070),
}

// Ceph CSI common container arguments
var CsiAddressContainerArg = fmt.Sprintf("--csi-address=%s", csiEndpoint)
var EndpointContainerArg = fmt.Sprintf("--endpoint=%s", csiEndpoint)
var CsiAddonsEndpointContainerArg = fmt.Sprintf("--csi-addons-endpoint=%s", csiAddonsEndpoint)
var CsiAddonsAddressContainerArg = fmt.Sprintf("--csi-addons-address=%s", csiAddonsEndpoint)
var CsiAddonsNodeIdContainerArg = fmt.Sprintf("--node-id=$(%s)", NodeIdEnvVar.Name)
var LeaderElectionContainerArg = "--leader-election=true"
var NodeIdContainerArg = fmt.Sprintf("--nodeid=$(%s)", NodeIdEnvVar.Name)
var PidlimitContainerArg = "--pidlimit=-1"
var ControllerServerContainerArg = "--controllerserver=true"
var NodeServerContainerArg = "--nodeserver=true"
var RetryIntervalStartContainerArg = "--retry-interval-start=500ms"
var DefaultFsTypeContainerArg = "--default-fstype=ext4"
var HandleVolumeInuseErrorContainerArg = "--handle-volume-inuse-error=false"
var PodUidContainerArg = fmt.Sprintf("--pod-uid=$(%s)", PodUidEnvVar.Name)
var PodContainerArg = fmt.Sprintf("--pod=$(%s)", PodNameEnvVar.Name)
var NamespaceContainerArg = fmt.Sprintf("--namespace=$(%s)", PodNamespaceEnvVar.Name)
var ControllerPortContainerArg = fmt.Sprintf("--controller-port=%d", CsiAddonsContainerPort.ContainerPort)
var DriverNamespaceContainerArg = fmt.Sprintf("--drivernamespace=$(%s)", DriverNamespaceEnvVar.Name)
var MetricsPathContainerArg = "--metricspath=/metrics"
var PoolTimeContainerArg = "--polltime=60s"
var ExtraCreateMetadataContainerArg = "--extra-create-metadata=true"
var PreventVolumeModeConversionContainerArg = "--prevent-volume-mode-conversion=true"
var HonorPVReclaimPolicyContainerArg = "--feature-gates=HonorPVReclaimPolicy=true"
var ImmediateTopologyContainerArg = "--immediate-topology=false"
var RecoverVolumeExpansionFailureContainerArg = "--feature-gates=RecoverVolumeExpansionFailure=true"
var EnableVolumeGroupSnapshotsContainerArg = "--enable-volume-group-snapshots=true"
var ForceCephKernelClientContainerArg = "--forcecephkernelclient=true"
var LogToStdErrContainerArg = "--logtostderr=false"
var AlsoLogToStdErrContainerArg = "--alsologtostderr=true"

func LogVerbosityContainerArg(level int) string {
	return fmt.Sprintf("--v=%d", Clamp(level, 0, 5))
}
func LogFileContainerArg(containerName string) string {
	return fmt.Sprintf("--log_file=csi-logs/%s.log", containerName)
}
func TypeContainerArg(t string) string {
	switch t {
	case "rbd", "cephfs", "nfs", "controller", "liveness":
		return fmt.Sprintf("--type=%s", t)
	default:
		return ""
	}
}
func SetMetadataContainerArg(on bool) string {
	return If(on, "--setmetadata=true", "")
}
func TimeoutContainerArg(timeout int) string {
	return fmt.Sprintf("--timeout=%ds", timeout)
}
func LeaderElectionNamespaceContainerArg(ns string) string {
	return If(ns != "", fmt.Sprintf("--leader-election-namespace=%s", ns), "")
}
func LeaderElectionLeaseDurationContainerArg(duration int) string {
	return fmt.Sprintf("--leader-election-lease-duration=%ds", duration)
}
func LeaderElectionRenewDeadlineContainerArg(deadline int) string {
	return fmt.Sprintf("--leader-election-renew-deadline=%ds", deadline)
}
func LeaderElectionRetryPeriodContainerArg(period int) string {
	return fmt.Sprintf("--leader-election-retry-period=%ds", period)
}
func DriverNameContainerArg(name string) string {
	return If(name != "", fmt.Sprintf("--drivername=%s", name), "")
}
func ClusterNameContainerArg(name string) string {
	return If(name != "", fmt.Sprintf("--clustername=%s", name), "")
}
func MetricsPortContainerArg(port int) string {
	return fmt.Sprintf("--metricsport=%d", port)
}
func KubeletRegistrationPathContainerArg(kubeletDirPath string, driverName string) string {
	return fmt.Sprintf("--kubelet-registration-path=%s/plugins/%s/csi.sock", kubeletDirPath, driverName)
}
func StagingPathContainerArg(kubeletDirPath string) string {
	return fmt.Sprintf("--stagingpath=%s/plugins/kubernetes.io/csi/", kubeletDirPath)
}
func LogRotateConfigMapName(driverName string) string {
	return fmt.Sprintf("%s-logrotate-config", driverName)
}
func KernelMountOptionsContainerArg(options map[string]string) string {
	return If(
		len(options) > 0,
		fmt.Sprintf("--kernelmountoptions==%s", MapToString(options, "=", ",")),
		"",
	)
}
func FuseMountOptionsContainerArg(options map[string]string) string {
	return If(
		len(options) == 0,
		fmt.Sprintf("--fusemountoptions==%s", MapToString(options, "=", ",")),
		"",
	)
}
func DomainLabelsContainerArg(options []string) string {
	return If(
		len(options) > 0,
		fmt.Sprintf("--domainlabels=%s", strings.Join(options, ",")),
		"",
	)
}
