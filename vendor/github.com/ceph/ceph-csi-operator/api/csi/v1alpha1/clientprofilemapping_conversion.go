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
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	csiv1beta1 "github.com/ceph/ceph-csi-operator/api/csi/v1beta1"
)

// convertBlockPoolIdPair is a generic convertor function to convert between
// different versions of BlockPoolIdPair types
func convertBlockPoolIdPair[From ~[]string, To ~[]string](mappings []From) []To {
	ret := make([]To, len(mappings))

	for i, pair := range mappings {
		ret[i] = To(slices.Clone(pair))
	}

	return ret
}

// ConvertTo converts this ClientProfileMapping (v1alpha1) to the Hub version (v1beta1).
func (src *ClientProfileMapping) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*csiv1beta1.ClientProfileMapping)
	if !ok {
		return errors.New("convertto: failed to cast to v1beta1 ClientProfileMapping")
	}

	log.Printf("ConvertTo: Converting ClientProfileMapping from Spoke version v1alpha1 to Hub version v1beta1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	dst.ObjectMeta = src.ObjectMeta

	if len(src.Spec.Mappings) > 0 {
		dst.Spec.Mappings = make([]csiv1beta1.MappingsSpec, len(src.Spec.Mappings))

		for i, mapping := range src.Spec.Mappings {
			dst.Spec.Mappings[i] = csiv1beta1.MappingsSpec{
				LocalClientProfile:  mapping.LocalClientProfile,
				RemoteClientProfile: mapping.RemoteClientProfile,
				BlockPoolIdMapping:  convertBlockPoolIdPair[BlockPoolIdPair, csiv1beta1.BlockPoolIdPair](mapping.BlockPoolIdMapping),
			}
		}
	}

	return nil
}

// ConvertFrom converts the Hub version (v1beta1) to this ClientProfileMapping (v1alpha1).
func (dst *ClientProfileMapping) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*csiv1beta1.ClientProfileMapping)
	if !ok {
		return errors.New("convertfrom: failed to cast to v1beta1 ClientProfileMapping")
	}

	log.Printf("ConvertFrom: Converting ClientProfileMapping from Hub version v1beta1 to Spoke version v1alpha1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	dst.ObjectMeta = src.ObjectMeta

	if len(src.Spec.Mappings) > 0 {
		dst.Spec.Mappings = make([]MappingsSpec, len(src.Spec.Mappings))

		for i, mapping := range src.Spec.Mappings {
			dst.Spec.Mappings[i] = MappingsSpec{
				LocalClientProfile:  mapping.LocalClientProfile,
				RemoteClientProfile: mapping.RemoteClientProfile,
				BlockPoolIdMapping:  convertBlockPoolIdPair[csiv1beta1.BlockPoolIdPair, BlockPoolIdPair](mapping.BlockPoolIdMapping),
			}
		}
	}

	return nil
}
