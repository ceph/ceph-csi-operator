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

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/utils/ptr"

	csiv1a1 "github.com/ceph/ceph-csi-operator/api/v1alpha1"
)

var driverDefaults = csiv1a1.DriverSpec{
	EnableMetadata:   ptr.To(false),
	GRpcTimeout:      150,
	SnapshotPolicy:   csiv1a1.AutoDetectSnapshotPolicy,
	GenerateOMapInfo: ptr.To(false),
	FsGroupPolicy:    storagev1.FileFSGroupPolicy,
	AttachRequired:   ptr.To(true),
	DeployCsiAddons:  ptr.To(false),
	CephFsClientType: csiv1a1.KernelCephFsClient,
	LeaderElection: &csiv1a1.LeaderElectionSpec{
		LeaseDuration: 137,
		RenewDeadline: 107,
		RetryPeriod:   26,
	},
	Plugin: &csiv1a1.PluginSpec{
		PodCommonSpec: csiv1a1.PodCommonSpec{
			PrioritylClassName: ptr.To(""),
		},
		EnableSeLinuxHostMount: ptr.To(false),
	},
	Provisioner: &csiv1a1.ProvisionerSpec{
		PodCommonSpec: csiv1a1.PodCommonSpec{
			PrioritylClassName: ptr.To(""),
		},
	},
}

var operatorNamespace = ""
var operatorConfigName = ""

func init() {
	ok := false

	if operatorNamespace, ok = os.LookupEnv("OPERATOR_NAMESPACE"); !ok {
		panic("Missing required OPERATOR_NAMESPACE environment variable")
	}

	if operatorConfigName, ok = os.LookupEnv("OPERATOR_CONFIG_NAME"); !ok {
		operatorConfigName = "ceph-csi-operator-config"
	}
}
