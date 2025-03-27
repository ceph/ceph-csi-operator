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

// ConvertTo converts this OperatorConfig (v1alpha1) to the Hub version (v1beta1).
func (src *OperatorConfig) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*csiv1beta1.OperatorConfig)
	if !ok {
		return errors.New("convertto: failed to cast to v1beta1 OperatorConfig")
	}

	log.Printf("ConvertTo: Converting OperatorConfig from Spoke version v1alpha1 to Hub version v1beta1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	dst.ObjectMeta = src.ObjectMeta

	if src.Spec.DriverSpecDefaults != nil {
		cSpec := ConvertAlphaDriverToBeta(*src.Spec.DriverSpecDefaults)
		dst.Spec.DriverSpecDefaults = &cSpec
	}

	return nil
}

// ConvertFrom converts the Hub version (v1beta1) to this OperatorConfig (v1alpha1).
func (dst *OperatorConfig) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*csiv1beta1.OperatorConfig)
	if !ok {
		return errors.New("convertto: failed to cast to v1beta1 OperatorConfig")
	}

	log.Printf("ConvertFrom: Converting OperatorConfig from Hub version v1beta1 to Spoke version v1alpha1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	dst.ObjectMeta = src.ObjectMeta

	if src.Spec.DriverSpecDefaults != nil {
		cSpec := ConvertBetaDriverToAlpha(*src.Spec.DriverSpecDefaults)
		dst.Spec.DriverSpecDefaults = &cSpec
	}
	return nil
}
