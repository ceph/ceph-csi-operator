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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Name Predicate return a predicate the filter events produced
// by resources that matches the given name
func NamePredicate(name string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == name
	})
}

// EventTypePredicate return a predicate the filter events based on their
// respective event type. This helper allows for the selection of multiple
// types resulting in a predicate that can filter in more then a single event
// type
func EventTypePredicate(create, update, del, generic bool) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return create
		},
		UpdateFunc: func(_ event.UpdateEvent) bool {
			return update
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return del
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return generic
		},
	}
}
