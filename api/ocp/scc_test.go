package ocp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecurityContextConstraints(t *testing.T) {
	testNamespace := "test-namespace"
	testName := "test"
	scc, err := NewSecurityContextConstraints(testName, testNamespace)
	require.NoError(t, err, "NewSecurityContextConstraints should not return an error")
	assert.NotNil(t, scc, "SCC should not be nil")

	assert.Equal(t, scc.Name, testName)
	assert.NotEmpty(t, scc.Users, "Users should not be empty")
	for _, user := range scc.Users {
		assert.True(t, strings.Contains(user, testNamespace),
			"Each user should contain the specified namespace")
		assert.False(t, strings.Contains(user, "{{.Namespace}}"),
			"Template placeholders should be replaced")
	}
}
