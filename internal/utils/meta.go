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
	"errors"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// AddAnnotation adds an annotation to a resource metadata, returns true if added else false
func AddAnnotation(obj metav1.Object, key string, value string) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
		obj.SetAnnotations(annotations)
	}
	if oldValue, exist := annotations[key]; !exist || oldValue != value {
		annotations[key] = value
		return true
	}
	return false
}

// IsOwnedBy returns true if the object has an owner ref for the provided owner
func IsOwnedBy(obj, owner metav1.Object) bool {
	ownerRefs := obj.GetOwnerReferences()
	for i := range ownerRefs {
		if owner.GetUID() == ownerRefs[i].UID {
			return true
		}
	}
	return false
}

// ToggleOwnerReference adds or remove an owner reference for the given owner based on the first argument.
// The function return true if the owner reference list had changed and false it it didn't
func ToggleOwnerReference(on bool, obj, owner metav1.Object, scheme *runtime.Scheme) (bool, error) {
	ownerRefExists := IsOwnedBy(obj, owner)
	if on {
		if !ownerRefExists {
			err := ctrlutil.SetOwnerReference(owner, obj, scheme)
			return err == nil, err
		}
	} else if ownerRefExists {
		err := ctrlutil.RemoveOwnerReference(owner, obj, scheme)
		return err == nil, err
	}
	return false, nil
}

func IsNotFoundWithName(err error, name string) bool {
	if !k8serrors.IsNotFound(err) {
		return false
	}
	status, ok := err.(k8serrors.APIStatus)
	return (ok || errors.As(err, &status)) &&
		status.Status().Details != nil &&
		status.Status().Details.Name == name
}
