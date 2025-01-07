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
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
)

const (
	operatorNamespaceEnvVar = "OPERATOR_NAMESPACE"
)

// RunConcurrently runs all the of the given functions concurrently returning a channel with
// the functions' return values (of type error) then closes the channel when all functions return.
func RunConcurrently(fnList ...func() error) chan error {
	errors := make(chan error)
	wg := sync.WaitGroup{}

	// Run all the functions concurrently
	for _, fn := range fnList {
		fn := fn
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- fn()
		}()
	}

	// Close the output channel whenever all of the functions completed
	go func() {
		wg.Wait()
		close(errors)
	}()

	// Read from the channel and aggregate into a slice
	return errors
}

// ChannelToSlice consumes a channel return values in a slice
func ChannelToSlice[T any](c chan T) []T {
	list := []T{}
	for value := range c {
		list = append(list, value)
	}
	return list
}

// If implements an if/else expression
func If[T any](cond bool, trueVal, falseVal T) T {
	if cond {
		return trueVal
	} else {
		return falseVal
	}
}

// Clamp a number between min and max
func Clamp[T cmp.Ordered](val, low, high T) T {
	if val < low {
		return low
	} else if val > high {
		return high
	} else {
		return val
	}
}

// MapSlice maps the iteas of a given slice into a new slice using a mapper function
func MapSlice[T, K any](in []T, mapper func(item T) K) []K {
	out := make([]K, len(in))
	for i := range in {
		out[i] = mapper(in[i])
	}
	return out
}

// MapToString serializes the provided map into a a string.
func MapToString[K, T ~string](m map[K]T, keyValueSeperator, itemSeperator string) string {
	if len(m) == 0 {
		return ""
	}

	// An item separator is added before each item. For the first item we want
	// the separator to be an empty string
	itemSep := ""
	bldr := strings.Builder{}
	for key, value := range m {
		bldr.WriteString(itemSep)
		bldr.WriteString(string(key))

		// Skip value serialization if it evaluates to an empty string
		valAsString := string(value)
		if valAsString != "" {
			bldr.WriteString(keyValueSeperator)
			bldr.WriteString(valAsString)
		}

		itemSep = itemSeperator
	}
	return bldr.String()
}

// Call calls the provided zero-argument function.
// This util is used whenever we need to define a function and call it immediately and only once,
// as a more readable alternative to (func() { ... })(). The common use case is "inline" func
// invoation as part of a data staructure initializtaoin code.
func Call[T any](fn func() T) T {
	return fn()
}

// RemoveZeroValues return a new slice form the provided slice where all zero-valued
// items are removed
func DeleteZeroValues[T comparable](slice []T) []T {
	var zero T
	return slices.DeleteFunc(slice, func(value T) bool {
		return value == zero
	})
}

func GetOperatorNamespace() (string, error) {
	ns := os.Getenv(operatorNamespaceEnvVar)
	if ns == "" {
		return "", fmt.Errorf("%s must be set", operatorNamespaceEnvVar)
	}
	return ns, nil
}
