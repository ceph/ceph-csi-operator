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

// ConvertTo converts this CephConnection (v1alpha1) to the Hub version (v1beta1).
func (src *CephConnection) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*csiv1beta1.CephConnection)
	if !ok {
		return errors.New("convertto: failed to cast to v1beta1 CephConnection")
	}

	log.Printf("ConvertTo: Converting CephConnection from Spoke version v1alpha1 to Hub version v1beta1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.Monitors = src.Spec.Monitors
	dst.Spec.RbdMirrorDaemonCount = src.Spec.RbdMirrorDaemonCount
	dst.Spec.ReadAffinity = (*csiv1beta1.ReadAffinitySpec)(src.Spec.ReadAffinity)

	return nil
}

// ConvertFrom converts the Hub version (v1beta1) to this CephConnection (v1alpha1).
func (dst *CephConnection) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*csiv1beta1.CephConnection)
	if !ok {
		return errors.New("convertfrom: failed to cast to v1beta1 CephConnection")

	}

	log.Printf("ConvertFrom: Converting CephConnection from Hub version v1beta1 to Spoke version v1alpha1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.Monitors = src.Spec.Monitors
	dst.Spec.RbdMirrorDaemonCount = src.Spec.RbdMirrorDaemonCount
	dst.Spec.ReadAffinity = (*ReadAffinitySpec)(src.Spec.ReadAffinity)

	return nil
}
