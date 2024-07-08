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

	csiv1a1 "github.com/ceph/ceph-csi-operator/api/v1alpha1"
)

var imageDefaults = map[string]string{
	"provisioner": "registry.k8s.io/sig-storage/csi-provisioner:v5.0.1",
	"attacher":    "registry.k8s.io/sig-storage/csi-attacher:v4.6.1",
	"resizer":     "registry.k8s.io/sig-storage/csi-resizer:v1.11.1",
	"snapshotter": "registry.k8s.io/sig-storage/csi-snapshotter:v8.0.1",
	"registrar":   "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.10.1",
	"plugin":      "quay.io/cephcsi/cephcsi:v3.10.2",
	"addons":      "quay.io/csiaddons/k8s-sidecar:v0.8.0",
}

const (
	defaultGRrpcTimeout   = 150
	defaultSnapshotPolicy = csiv1a1.AutoDetectSnapshotPolicy
)

var defaultLeaderElection = csiv1a1.LeaderElectionSpec{
	LeaseDuration: 137,
	RenewDeadline: 107,
	RetryPeriod:   26,
}

var operatorNamespace = (func() string {
	namespace, _ := os.LookupEnv("OPERATOR_NAMESPACE")
	if namespace == "" {
		panic("Required OPERATOR_NAMESPACE environment variable is either missing or empty")
	}
	return namespace
})()

var operatorConfigName = (func() string {
	name, ok := os.LookupEnv("OPERATOR_CONFIG_NAME")
	if ok {
		if name == "" {
			panic("OPERATOR_CONFIG_NAME exists but empty")
		}
		return name
	}
	return "ceph-csi-operator-config"
})()
