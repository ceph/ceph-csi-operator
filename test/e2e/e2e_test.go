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

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ceph/ceph-csi-operator/test/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	csiv1alpha1 "github.com/ceph/ceph-csi-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const namespace = "ceph-csi-operator-system"

var k8sClient client.Client
var clientset *kubernetes.Clientset

var _ = Describe("Ceph CSI Operator", Ordered, func() {
	BeforeAll(func() {
		var err error
		cfg, err := config.GetConfig()
		Expect(err).NotTo(HaveOccurred())

		err = csiv1alpha1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())

		clientset, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		By("creating manager namespace")
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		err = k8sClient.Create(context.TODO(), ns)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("removing manager namespace")
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		err := k8sClient.Delete(context.TODO(), ns)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Operator Deployment", func() {
		It("should deploy and run successfully", func() {
			var err error

			projectimage := "example.com/ceph-csi-operator:v0.0.1"

			By("building the operator image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("loading the operator image to the cluster")
			err = utils.LoadImageToCluster(projectimage)
			Expect(err).NotTo(HaveOccurred())

			By("installing CRDs")
			cmd = exec.Command("make", "install")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("deploying the operator")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("validating that the operator pod is running")
			Eventually(func() error {
				pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
					LabelSelector: "control-plane=ceph-csi-op-controller-manager",
				})
				if err != nil {
					return err
				}
				if len(pods.Items) != 1 {
					return fmt.Errorf("expected 1 operator pod, but got %d", len(pods.Items))
				}
				if pods.Items[0].Status.Phase != corev1.PodRunning {
					return fmt.Errorf("operator pod is in %s status", pods.Items[0].Status.Phase)
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	Context("Driver CR", func() {
		It("should create a RBD Driver CR and verify resources", func() {
			driver := &csiv1alpha1.Driver{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "csi.ceph.io/v1alpha1",
					Kind:       "Driver",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test.rbd.csi.ceph.com",
					Namespace: namespace,
				},
				Spec: csiv1alpha1.DriverSpec{
					AttachRequired:   ptr.To(true),
					NodePlugin:       &csiv1alpha1.NodePluginSpec{},
					ControllerPlugin: &csiv1alpha1.ControllerPluginSpec{},
				},
			}

			By("creating a Driver CR")
			Expect(k8sClient.Create(context.TODO(), driver)).To(Succeed())

			By("verifying the Driver daemonset")
			Eventually(func() error {
				daemonset := &appsv1.DaemonSet{}
				err := k8sClient.Get(context.TODO(), client.ObjectKey{Name: "test.rbd.csi.ceph.com-nodeplugin", Namespace: namespace}, daemonset)
				if err != nil {
					return err
				}
				if daemonset.Status.NumberReady != daemonset.Status.DesiredNumberScheduled {
					return fmt.Errorf("daemonset not ready, expected %d nodes, got %d", daemonset.Status.DesiredNumberScheduled, daemonset.Status.NumberReady)
				}
				return nil
			}, time.Minute*5, time.Second*10).Should(Succeed())
		})
	})
})
