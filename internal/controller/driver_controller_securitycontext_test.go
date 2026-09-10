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

package controller

import (
	"context"
	"io"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	csiv1 "github.com/ceph/ceph-csi-operator/api/v1"
)

// newCtrlPluginTestReconcile returns a driverReconcile backed by a fake
// client for a driver with the given spec, so that generated resources can
// be asserted without a running API server.
func newCtrlPluginTestReconcile(t *testing.T, driver *csiv1.Driver) *driverReconcile {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add client-go types to scheme: %v", err)
	}
	if err := csiv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add csi types to scheme: %v", err)
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(driver).Build()

	return &driverReconcile{
		DriverReconciler: DriverReconciler{
			Client: c,
			Scheme: scheme,
		},
		ctx:        context.Background(),
		log:        zap.New(zap.WriteTo(io.Discard)),
		driver:     *driver,
		driverType: RbdDriverType,
		images: map[string]string{
			"plugin": "quay.io/cephcsi/cephcsi:test",
			"addons": "quay.io/csiaddons/k8s-sidecar:test",
		},
	}
}

// fetchCtrlPluginDeployment reconciles the controller plugin deployment and
// returns it together with the container of the given name.
func fetchCtrlPluginDeployment(
	t *testing.T,
	r *driverReconcile,
	containerName string,
) (*appsv1.Deployment, *corev1.Container) {
	t.Helper()

	if err := r.reconcileControllerPluginDeployment(); err != nil {
		t.Fatalf("failed to reconcile controller plugin deployment: %v", err)
	}

	deploy := &appsv1.Deployment{}
	err := r.Get(
		r.ctx,
		types.NamespacedName{
			Name:      r.generateName("ctrlplugin"),
			Namespace: r.driver.Namespace,
		},
		deploy,
	)
	if err != nil {
		t.Fatalf("failed to get controller plugin deployment: %v", err)
	}

	for i := range deploy.Spec.Template.Spec.Containers {
		if deploy.Spec.Template.Spec.Containers[i].Name == containerName {
			return deploy, &deploy.Spec.Template.Spec.Containers[i]
		}
	}
	t.Fatalf("container %q not found in controller plugin deployment", containerName)
	return nil, nil
}

func assertPrivileged(t *testing.T, container *corev1.Container) {
	t.Helper()

	if container.SecurityContext == nil ||
		container.SecurityContext.Privileged == nil ||
		!*container.SecurityContext.Privileged {
		t.Errorf(
			"expected container %q to run privileged, got securityContext %+v",
			container.Name,
			container.SecurityContext,
		)
	}
}

// A driver as rendered by the helm chart defaults has log rotation enabled
// while the controller plugin is not privileged. On hosts with SELinux in
// enforcing mode, an unprivileged container cannot write to the hostPath
// logs dir, hence the containers writing the log files must run privileged
// irrespective of the privileged field.
func TestControllerPluginPrivilegedWithLogRotation(t *testing.T) {
	driver := &csiv1.Driver{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test.rbd.csi.ceph.com",
			Namespace: "default",
		},
		Spec: csiv1.DriverSpec{
			Log: &csiv1.LogSpec{
				Rotation: &csiv1.LogRotationSpec{
					MaxFiles:    7,
					MaxLogSize:  resource.MustParse("10G"),
					Periodicity: csiv1.DailyPeriod,
				},
			},
			ControllerPlugin: &csiv1.ControllerPluginSpec{
				Privileged: ptr.To(false),
			},
			DeployCsiAddons: ptr.To(true),
		},
	}
	r := newCtrlPluginTestReconcile(t, driver)

	deploy, pluginContainer := fetchCtrlPluginDeployment(t, r, "csi-rbdplugin")
	assertPrivileged(t, pluginContainer)

	_, logRotatorContainer := fetchCtrlPluginDeployment(t, r, "log-rotator")
	assertPrivileged(t, logRotatorContainer)

	_, csiAddonsContainer := fetchCtrlPluginDeployment(t, r, "csi-addons")
	assertPrivileged(t, csiAddonsContainer)

	logsDirFound := false
	for _, vol := range deploy.Spec.Template.Spec.Volumes {
		if vol.Name == "logs-dir" {
			logsDirFound = true
			expectedPath := defaultLogHostPath + "/" + deploy.Name
			if vol.HostPath == nil || vol.HostPath.Path != expectedPath {
				t.Errorf(
					"expected logs-dir hostPath %q, got %+v",
					expectedPath,
					vol.HostPath,
				)
			}
		}
	}
	if !logsDirFound {
		t.Error("expected logs-dir volume in controller plugin deployment")
	}
}

// Even without the privileged field set, e.g. for drivers created directly
// against the operator API, log rotation requires the controller plugin
// containers to run privileged.
func TestControllerPluginPrivilegedDefaultsToTrueWithLogRotation(t *testing.T) {
	driver := &csiv1.Driver{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test.rbd.csi.ceph.com",
			Namespace: "default",
		},
		Spec: csiv1.DriverSpec{
			Log: &csiv1.LogSpec{
				Rotation: &csiv1.LogRotationSpec{
					MaxFiles:    7,
					MaxLogSize:  resource.MustParse("10G"),
					Periodicity: csiv1.DailyPeriod,
				},
			},
		},
	}
	r := newCtrlPluginTestReconcile(t, driver)

	_, pluginContainer := fetchCtrlPluginDeployment(t, r, "csi-rbdplugin")
	assertPrivileged(t, pluginContainer)

	_, logRotatorContainer := fetchCtrlPluginDeployment(t, r, "log-rotator")
	assertPrivileged(t, logRotatorContainer)
}

// Without log rotation no log files are written to the hostPath logs dir and
// the controller plugin containers keep running unprivileged.
func TestControllerPluginUnprivilegedWithoutLogRotation(t *testing.T) {
	driver := &csiv1.Driver{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test.rbd.csi.ceph.com",
			Namespace: "default",
		},
		Spec: csiv1.DriverSpec{
			Log: &csiv1.LogSpec{
				Verbosity: 1,
			},
			ControllerPlugin: &csiv1.ControllerPluginSpec{
				Privileged: ptr.To(false),
			},
		},
	}
	r := newCtrlPluginTestReconcile(t, driver)

	deploy, pluginContainer := fetchCtrlPluginDeployment(t, r, "csi-rbdplugin")
	if pluginContainer.SecurityContext != nil {
		t.Errorf(
			"expected container %q to have no securityContext, got %+v",
			pluginContainer.Name,
			pluginContainer.SecurityContext,
		)
	}

	for _, container := range deploy.Spec.Template.Spec.Containers {
		if container.Name == "log-rotator" {
			t.Error("unexpected log-rotator container without log rotation")
		}
	}
	for _, vol := range deploy.Spec.Template.Spec.Volumes {
		if vol.Name == "logs-dir" {
			t.Error("unexpected logs-dir volume without log rotation")
		}
	}
}
