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
)

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

func ChannelToSlice[T any](c chan T) []T {
	list := []T{}
	for value := range c {
		list = append(list, value)
	}
	return list
}
