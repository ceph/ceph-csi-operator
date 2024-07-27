package ocp

import (
	_ "embed"
	"fmt"
	"strings"

	secv1 "github.com/openshift/api/security/v1"
	"sigs.k8s.io/yaml"
)

//go:embed scc.yaml
var sccYAMLTemplate string

// NewSecurityContextConstraints loads the embedded SCC YAML template, replaces the namespace,
// and returns it as a SecurityContextConstraints object
func NewSecurityContextConstraints(name string, namespace string) (*secv1.SecurityContextConstraints, error) {
	scc := &secv1.SecurityContextConstraints{}
	err := yaml.Unmarshal([]byte(sccYAMLTemplate), scc)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling YAML: %v", err)
	}
	scc.Name = name
	scc.Namespace = namespace
	var users []string
	for _, user := range scc.Users {
		serviceAccount := strings.Split(user, ":")
		if len(serviceAccount) != 4 {
			return nil, fmt.Errorf("invalid service account name")
		}
		serviceAccount[2] = namespace
		users = append(users, strings.Join(serviceAccount, ":"))
	}
	scc.Users = users
	return scc, nil
}
