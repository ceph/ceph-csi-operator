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
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testValueNew  = "new"
	testValueKeep = "keep"
)

// Dummy source and destination structs for testing
type sourceItem struct {
	ID    string
	Value string
}

type destItem struct {
	ID    string
	Value string
}

func TestMapMergeByKey(t *testing.T) {
	tests := []struct {
		name     string
		dest     []destItem
		src      []sourceItem
		expected []destItem
	}{
		{
			name: "append new items",
			dest: []destItem{},
			src: []sourceItem{
				{ID: "a", Value: "one"},
				{ID: "b", Value: "two"},
			},
			expected: []destItem{
				{ID: "a", Value: "one"},
				{ID: "b", Value: "two"},
			},
		},
		{
			name: "replace existing item",
			dest: []destItem{
				{ID: "a", Value: "old"},
			},
			src: []sourceItem{
				{ID: "a", Value: testValueNew},
			},
			expected: []destItem{
				{ID: "a", Value: testValueNew},
			},
		},
		{
			name: "append and replace mix",
			dest: []destItem{
				{ID: "x", Value: testValueKeep},
				{ID: "a", Value: "old"},
			},
			src: []sourceItem{
				{ID: "a", Value: testValueNew},
				{ID: "b", Value: "added"},
			},
			expected: []destItem{
				{ID: "x", Value: testValueKeep},
				{ID: "a", Value: testValueNew},
				{ID: "b", Value: "added"},
			},
		},
		{
			name: "skip empty keys",
			dest: []destItem{},
			src: []sourceItem{
				{ID: "", Value: "skip"},
				{ID: "b", Value: testValueKeep},
			},
			expected: []destItem{
				{ID: "b", Value: testValueKeep},
			},
		},
		{
			name: "skip transform=false",
			dest: []destItem{},
			src: []sourceItem{
				{ID: "a", Value: "ignore"},
				{ID: "b", Value: "ok"},
			},
			expected: []destItem{
				{ID: "b", Value: "ok"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapMergeByKey(
				tt.dest,
				tt.src,
				func(s sourceItem) (destItem, bool) {
					// skip item with Value "ignore"
					if s.Value == "ignore" {
						return destItem{}, false
					}
					return destItem(s), true
				},
				func(d destItem) string {
					return d.ID
				},
			)

			assert.Equal(t, tt.expected, got)
		})
	}
}
