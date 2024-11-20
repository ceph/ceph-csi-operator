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

package controller

import (
	"os"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	csiv1a1 "github.com/ceph/ceph-csi-operator/api/v1alpha1"
)

var imageDefaults = map[string]string{
	"provisioner": "registry.k8s.io/sig-storage/csi-provisioner:v5.0.1",
	"attacher":    "registry.k8s.io/sig-storage/csi-attacher:v4.6.1",
	"resizer":     "registry.k8s.io/sig-storage/csi-resizer:v1.11.1",
	"snapshotter": "registry.k8s.io/sig-storage/csi-snapshotter:v8.0.1",
	"registrar":   "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.11.1",
	"plugin":      "quay.io/cephcsi/cephcsi:v3.12.2",
	"addons":      "quay.io/csiaddons/k8s-sidecar:v0.10.0",
}

const (
	defaultGRrpcTimeout      = 150
	defaultKubeletDirPath    = "/var/lib/kubelet"
	defaultLogHostPath       = "/var/lib/cephcsi"
	defaultLogRotateMaxFiles = 7
)

var defaultLeaderElection = csiv1a1.LeaderElectionSpec{
	LeaseDuration: 137,
	RenewDeadline: 107,
	RetryPeriod:   26,
}

var defaultDaemonSetUpdateStrategy = appsv1.DaemonSetUpdateStrategy{
	Type: appsv1.RollingUpdateDaemonSetStrategyType,
	RollingUpdate: &appsv1.RollingUpdateDaemonSet{
		MaxUnavailable: ptr.To(intstr.FromInt(1)),
	},
}

var defaultDeploymentStrategy = appsv1.DeploymentStrategy{
	Type: appsv1.RollingUpdateDeploymentStrategyType,
	RollingUpdate: &appsv1.RollingUpdateDeployment{
		MaxSurge:       ptr.To(intstr.FromString("25%")),
		MaxUnavailable: ptr.To(intstr.FromString("25%")),
	},
}

var (
	operatorNamespace    string
	operatorConfigName   string
	serviceAccountPrefix string
)

func InitConfig() {
	if operatorNamespace = os.Getenv("OPERATOR_NAMESPACE"); operatorNamespace == "" {
		panic("Required OPERATOR_NAMESPACE environment variable is either missing or empty")
	}

	serviceAccountPrefix = os.Getenv("CSI_SERVICE_ACCOUNT_PREFIX")

	envOperatorConfigName, set := os.LookupEnv("OPERATOR_CONFIG_NAME")
	if set {
		if envOperatorConfigName == "" {
			panic("OPERATOR_CONFIG_NAME exists but empty")
		}
		operatorConfigName = envOperatorConfigName
	} else {
		operatorConfigName = "ceph-csi-operator-config"
	}
}
