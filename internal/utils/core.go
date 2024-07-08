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
	"sync"

	"golang.org/x/exp/constraints"
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

// FirstNonNil returns the first non nil argument or nil if all arguments are nil
func FirstNonNil[T any](ptrs ...*T) *T {
	for _, ptr := range ptrs {
		if ptr != nil {
			return ptr
		}
	}
	return nil
}

// FirstNonEmpty returns the first non empty string or an empty string if all
// arguments are empty strings
func FirstNonEmpty[T ~string](strings ...T) T {
	for _, str := range strings {
		if str != "" {
			return str
		}
	}
	return ""
}

func FirstNonZero[T constraints.Integer](numbers ...T) T {
	for _, num := range numbers {
		if num != 0 {
			return num
		}
	}
	return 0
}

// Clamp a number between min and max
func Clamp[T constraints.Ordered](val, low, high T) T {
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
