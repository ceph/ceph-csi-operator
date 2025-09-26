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
	"reflect"
	"testing"
)

// Not using table like test cases as `MapFilter` is generic
// And thus would require reflections.
func TestMapFilter_IntToString(t *testing.T) {
	in := []int{1, 2, 3, 4, 5}
	mapper := func(n int) (string, bool) {
		if n%2 == 0 {
			return "even", true
		}
		return "", false
	}

	got := MapFilter(in, mapper)
	want := []string{"even", "even"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("TestMapFilter_IntToString() = %v, want %v", got, want)
	}
}

func TestMapFilter_EmptyInput(t *testing.T) {
	in := []int{}
	mapper := func(n int) (int, bool) {
		return n * 2, true
	}

	got := MapFilter(in, mapper)
	want := []int{}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("TestMapFilter_EmptyInput() = %v, want %v", got, want)
	}
}

func TestMapFilter_AllFilteredOut(t *testing.T) {
	in := []string{"a", "bb", "ccc"}
	mapper := func(s string) (int, bool) {
		// Only include strings of length > 3
		if len(s) > 3 {
			return len(s), true
		}
		return 0, false
	}

	got := MapFilter(in, mapper)
	want := []int{}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("TestMapFilter_AllFilteredOut() = %v, want %v", got, want)
	}
}

func TestMapFilter_KeepAll(t *testing.T) {
	in := []int{1, 2, 3}
	mapper := func(n int) (int, bool) {
		return n * n, true
	}

	got := MapFilter(in, mapper)
	want := []int{1, 4, 9}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("TestMapFilter_KeepAll() = %v, want %v", got, want)
	}
}

func TestMapFilter_BooleanMapping(t *testing.T) {
	in := []string{"go", "lang", "gopher"}
	mapper := func(s string) (bool, bool) {
		return len(s) > 2, true
	}

	got := MapFilter(in, mapper)
	want := []bool{false, true, true}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("TestMapFilter_BooleanMapping() = %v, want %v", got, want)
	}
}
