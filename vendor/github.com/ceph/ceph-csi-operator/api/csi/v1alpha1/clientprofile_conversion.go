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

// ConvertTo converts this ClientProfile (v1alpha1) to the Hub version (v1beta1).
func (src *ClientProfile) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*csiv1beta1.ClientProfile)
	if !ok {
		return errors.New("convertto: failed to cast to v1beta1 ClientProfile")
	}

	log.Printf("ConvertTo: Converting ClientProfile from Spoke version v1alpha1 to Hub version v1beta1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.CephConnectionRef = src.Spec.CephConnectionRef
	dst.Spec.CephFs = (*csiv1beta1.CephFsConfigSpec)(src.Spec.CephFs)
	dst.Spec.Nfs = (*csiv1beta1.NfsConfigSpec)(src.Spec.Nfs)
	dst.Spec.Rbd = (*csiv1beta1.RbdConfigSpec)(src.Spec.Rbd)

	return nil
}

// ConvertFrom converts the Hub version (v1beta1) to this ClientProfile (v1alpha1).
func (dst *ClientProfile) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*csiv1beta1.ClientProfile)
	if !ok {
		return errors.New("convertfrom: failed to cast to v1beta1 ClientProfile")
	}

	log.Printf("ConvertFrom: Converting ClientProfile from Hub version v1beta1 to Spoke version v1alpha1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.CephConnectionRef = src.Spec.CephConnectionRef
	dst.Spec.CephFs = (*CephFsConfigSpec)(src.Spec.CephFs)
	dst.Spec.Nfs = (*NfsConfigSpec)(src.Spec.Nfs)
	dst.Spec.Rbd = (*RbdConfigSpec)(src.Spec.Rbd)

	return nil
}
