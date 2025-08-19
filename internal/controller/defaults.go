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

	csiv1 "github.com/ceph/ceph-csi-operator/api/v1"
	"github.com/ceph/ceph-csi-operator/internal/utils"
)

var imageDefaults = map[string]string{
	"provisioner":       "registry.k8s.io/sig-storage/csi-provisioner:v5.3.0",
	"attacher":          "registry.k8s.io/sig-storage/csi-attacher:v4.9.0",
	"resizer":           "registry.k8s.io/sig-storage/csi-resizer:v1.13.2",
	"snapshotter":       "registry.k8s.io/sig-storage/csi-snapshotter:v8.2.0",
	"registrar":         "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.14.0",
	"snapshot-metadata": "registry.k8s.io/sig-storage/csi-snapshot-metadata:v0.1.0",
	"plugin":            "quay.io/cephcsi/cephcsi:v3.15.0",
	"addons":            "quay.io/csiaddons/k8s-sidecar:v0.13.0",
}

const (
	defaultGRrpcTimeout      = 150
	defaultKubeletDirPath    = "/var/lib/kubelet"
	defaultLogHostPath       = "/var/lib/cephcsi"
	defaultLogRotateMaxFiles = 7
)

var defaultLeaderElection = csiv1.LeaderElectionSpec{
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

var operatorNamespace = utils.Call(func() string {
	namespace, err := utils.GetOperatorNamespace()
	if err != nil {
		panic("Required OPERATOR_NAMESPACE environment variable is either missing or empty")
	}
	return namespace
})

var operatorConfigName = utils.Call(func() string {
	name, ok := os.LookupEnv("OPERATOR_CONFIG_NAME")
	if ok {
		if name == "" {
			panic("OPERATOR_CONFIG_NAME exists but empty")
		}
		return name
	}
	return "ceph-csi-operator-config"
})

var serviceAccountPrefix = utils.Call(func() string {
	return os.Getenv("CSI_SERVICE_ACCOUNT_PREFIX")
})
