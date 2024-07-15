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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LogSpec struct {
	// Log level for driver pods,
	// Supported values from 0 to 5. 0 for general useful logs (the default), 5 for trace level verbosity.
	// Default to 0
	LogLevel   int               `json:"logLevel,omitempty"`
	MaxFiles   int               `json:"maxFiles,omitempty"`
	MaxLogSize resource.Quantity `json:"maxLogSize,omitempty"`
}

type SnapshotPolicyType string

const (
	// Disables the feature and remove the snapshotter sidercar
	NoneSnapshotPolicy SnapshotPolicyType = "none"

	// Inspect the CRD's for volumesnapshot and volumegroupsnapshot and enable
	// corresponding features (will results in deployment of a snapshotter sidecar)
	AutoDetectSnapshotPolicy SnapshotPolicyType = "autodetect"

	// Enable the volumegroupsnapshot feature (will results in deployment of a snapshotter sidecar)
	VolumeGroupSnapshotPolicy SnapshotPolicyType = "volumeGroupSnapshot"

	// Enable the volumesnapshot feature (will results in deployment of a snapshotter sidecar)
	VolumeSnapshotSnapshotPolicy SnapshotPolicyType = "volumeSnapshot"
)

type EncryptionSpec struct {
	ConfigMapRef corev1.LocalObjectReference `json:"configMapName,omitempty"`
}

type PodCommonSpec struct {
	// Pod's user defined priority class name
	PrioritylClassName *string `json:"priorityClassName,omitempty"`

	// Pod's labels
	Labels map[string]string `json:"labels,omitempty"`

	// Pod's annotations
	Annotations map[string]string `json:"annotations,omitempty"`

	// Pod's affinity settings
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Pod's tolerations list
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

type NodePluginResourcesSpec struct {
	Registrar *corev1.ResourceRequirements `json:"registrar,omitempty"`
	Liveness  *corev1.ResourceRequirements `json:"liveness,omitempty"`
	Plugin    *corev1.ResourceRequirements `json:"plugin,omitempty"`
}

type NodePluginSpec struct {
	// Embedded common pods spec
	PodCommonSpec `json:"inline"`

	// Driver's plugin daemonset update strategy, supported values are OnDelete and RollingUpdate.
	// Default value is RollingUpdate with MaxAvailabile set to 1
	UpdateStrategy *appsv1.DaemonSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// Resource requirements for plugin's containers
	Resources NodePluginResourcesSpec `json:"resources,omitempty"`

	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// kubelet directory path, if kubelet configured to use other than /var/lib/kubelet path.
	KubeletDirPath string `json:"kubeletDirPath,omitempty"`

	// Control the host mount of /etc/selinux for csi plugin pods. Defaults to false
	EnableSeLinuxHostMount *bool `json:"EnableSeLinuxHostMount,omitempty"`

	// To indicate the image pull policy to be applied to all the containers in the csi driver pods.
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`
}

type ControllerPluginResourcesSpec struct {
	Attacher      *corev1.ResourceRequirements `json:"attacher,omitempty"`
	Snapshotter   *corev1.ResourceRequirements `json:"snapshotter,omitempty"`
	Resizer       *corev1.ResourceRequirements `json:"resizer,omitempty"`
	Provisioner   *corev1.ResourceRequirements `json:"provisioner,omitempty"`
	OMapGenerator *corev1.ResourceRequirements `json:"omapGenerator,omitempty"`
	Liveness      *corev1.ResourceRequirements `json:"liveness,omitempty"`
	Plugin        *corev1.ResourceRequirements `json:"plugin,omitempty"`
}

type ControllerPluginSpec struct {
	// Embedded common pods spec
	PodCommonSpec `json:"inline"`

	// Set replicas for controller plugin's deployment. Defaults to 2
	Replicas *int32 `json:"replicas,omitempty"`

	// Resource requirements for controller plugin's containers
	Resources ControllerPluginResourcesSpec `json:"resources,omitempty"`
}

type LivenessSpec struct {
	// Port to expose liveness metrics
	MetricsPort int `json:"metricsPort,omitempty"`
}

type LeaderElectionSpec struct {
	// Duration in seconds that non-leader candidates will wait to force acquire leadership.
	// Default to 137 seconds.
	LeaseDuration int `json:"leaseDuration,omitempty"`

	// Deadline in seconds that the acting leader will retry refreshing leadership before giving up.
	// Defaults to 107 seconds.
	RenewDeadline int `json:"renewDeadline,omitempty"`

	// Retry Period in seconds the LeaderElector clients should wait between tries of actions.
	// Defaults to 26 seconds.
	RetryPeriod int `json:"retryPeriod,omitempty"`
}

type CephFsClientType string

const (
	KernelCephFsClient CephFsClientType = "kernel"
	FuseCephFsClient   CephFsClientType = "fuse"
)

// DriverSpec defines the desired state of Driver
type DriverSpec struct {
	// Logging configuration for driver's pods
	Log *LogSpec `json:"log,omitempty"`

	// A reference to a ConfigMap resource holding image overwrite for deployed
	// containers
	ImageSet *corev1.LocalObjectReference `json:"imageSet,omitempty"`

	// Cluster name identifier to set as metadata on the CephFS subvolume and RBD images. This will be useful in cases
	// when two container orchestrator clusters (Kubernetes/OCP) are using a single ceph cluster.
	ClusterName *string `json:"clusterName,omitempty"`

	// Set to true to enable adding volume metadata on the CephFS subvolumes and RBD images.
	// Not all users might be interested in getting volume/snapshot details as metadata on CephFS subvolume and RBD images.
	// Hence enable metadata is false by default.
	EnableMetadata *bool `json:"enableMetadata,omitempty"`

	// Set the gRPC timeout for gRPC call issued by the driver components
	GRpcTimeout int `json:"grpcTimeout,omitempty"`

	// Select a policy for snapshot behavior: none, autodetect, snapshot, sanpshotGroup
	SnapshotPolicy SnapshotPolicyType `json:"snapshotPolicy,omitempty"`

	// OMAP generator will generate the omap mapping between the PV name and the RBD image.
	// Need to be enabled when we are using rbd mirroring feature.
	// By default OMAP generator sidecar is deployed with Csi controller plugin pod, to disable
	// it set it to false.
	GenerateOMapInfo *bool `json:"generateOMapInfo,omitempty"`

	// Policy for modifying a volume's ownership or permissions when the PVC is being mounted.
	// supported values are documented at https://kubernetes-csi.github.io/docs/support-fsgroup.html
	FsGroupPolicy storagev1.FSGroupPolicy `json:"fsGroupPolicy,omitempty"`

	// Driver's encryption settings
	Encryption *EncryptionSpec `json:"encryption,omitempty"`

	// Driver's plugin configuration
	NodePlugin *NodePluginSpec `json:"nodePlugin,omitempty"`

	// Driver's controller plugin configuration
	ControllerPlugin *ControllerPluginSpec `json:"controllerPlugin,omitempty"`

	// Whether to skip any attach operation altogether for CephCsi PVCs.
	// See more details [here](https://kubernetes-csi.github.io/docs/skip-attach.html#skip-attach-with-csi-driver-object).
	// If set to false it skips the volume attachments and makes the creation of pods using the CephCsi PVC fast.
	// **WARNING** It's highly discouraged to use this for RWO volumes. for RBD PVC it can cause data corruption,
	// csi-addons operations like Reclaimspace and PVC Keyrotation will also not be supported if set to false
	// since we'll have no VolumeAttachments to determine which node the PVC is mounted on.
	// Refer to this [issue](https://github.com/kubernetes/kubernetes/issues/103305) for more details.
	AttachRequired *bool `json:"attachRequired,omitempty"`

	// Liveness metrics configuration.
	// disabled by default.
	Liveness *LivenessSpec `json:"liveness,omitempty"`

	// Leader election setting
	LeaderElection *LeaderElectionSpec `json:"leaderElection,omitempty"`

	// TODO: do we want Csi addon specific field? or should we generalize to
	// a list of additional sidecars?
	DeployCsiAddons *bool `json:"deployCsiAddons,omitempty"`

	// Select between between cephfs kernel driver and ceph-fuse
	// If you select a non-kernel client, your application may be disrupted during upgrade.
	// See the upgrade guide: https://rook.io/docs/rook/latest/ceph-upgrade.html
	// NOTE! cephfs quota is not supported in kernel version < 4.17
	CephFsClientType CephFsClientType `json:"cephFsClientType,omitempty"`

	// Set mount options to use https://docs.ceph.com/en/latest/man/8/mount.ceph/#options
	// Set to "ms_mode=secure" when connections.encrypted is enabled in Ceph
	KernelMountOptions map[string]string `json:"kernelMountOptions,omitempty"`

	// Set mount options to use when using the Fuse client
	FuseMountOptionss map[string]string `json:"fuseMountOptions,omitempty"`
}

type DriverPhaseType string

const (
	ReadyDriverPhase DriverPhaseType = "Ready"
)

type DriverReasonType string

// TODO: Add failure reason codes
const ()

// DriverStatus defines the observed state of Driver
type DriverStatus struct {
	// TODO: Consider to move away from a single phase to a conditions based approach
	// or the a Ready list approach. Main reason this reconciler address multiple

	// The last known state of the latest reconcile
	Phase DriverPhaseType `json:"phase,omitempty"`

	// The reason for the last transition change.
	Reason DriverReasonType `json:"reason,omitempty"`

	// A human readable message indicating details about the last transition.
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Driver is the Schema for the drivers API
type Driver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DriverSpec   `json:"spec,omitempty"`
	Status DriverStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DriverList contains a list of Driver
type DriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Driver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Driver{}, &DriverList{})
}
