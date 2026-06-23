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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	csiv1 "github.com/ceph/ceph-csi-operator/api/v1"
)

var _ = Describe("Driver Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test.rbd.csi.ceph.com"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		driver := &csiv1.Driver{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Driver")
			err := k8sClient.Get(ctx, typeNamespacedName, driver)
			if err != nil && errors.IsNotFound(err) {
				resource := &csiv1.Driver{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &csiv1.Driver{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Driver")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &DriverReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("getControllerPluginReplicas", func() {
		var (
			reconciler driverReconcile
			log        = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
		)

		BeforeEach(func() {
			reconciler = driverReconcile{
				DriverReconciler: DriverReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				},
				ctx: context.Background(),
				log: log,
			}
		})

		It("should return specReplicas when explicitly set", func() {
			specReplicas := ptr.To(int32(5))
			result := reconciler.getControllerPluginReplicas(log, specReplicas)
			Expect(result).To(Equal(specReplicas))
		})

		It("should return default replicas when no nodes exist", func() {
			result := reconciler.getControllerPluginReplicas(log, nil)
			Expect(*result).To(Equal(defaultControllerPluginReplicas))
		})

		It("should cap replicas to 1 on a single-node cluster", func() {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "single-node",
				},
			}
			Expect(k8sClient.Create(context.Background(), node)).To(Succeed())
			defer func() {
				Expect(k8sClient.Delete(context.Background(), node)).To(Succeed())
			}()

			result := reconciler.getControllerPluginReplicas(log, nil)
			Expect(*result).To(Equal(int32(1)))
		})

		It("should return default replicas when node count meets default", func() {
			nodes := make([]*corev1.Node, defaultControllerPluginReplicas)
			for i := range nodes {
				nodes[i] = &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d", i),
					},
				}
				Expect(k8sClient.Create(context.Background(), nodes[i])).To(Succeed())
			}
			defer func() {
				for _, n := range nodes {
					Expect(k8sClient.Delete(context.Background(), n)).To(Succeed())
				}
			}()

			result := reconciler.getControllerPluginReplicas(log, nil)
			Expect(*result).To(Equal(defaultControllerPluginReplicas))
		})

		It("should not cap when specReplicas is set even on single-node cluster", func() {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "only-node",
				},
			}
			Expect(k8sClient.Create(context.Background(), node)).To(Succeed())
			defer func() {
				Expect(k8sClient.Delete(context.Background(), node)).To(Succeed())
			}()

			specReplicas := ptr.To(int32(3))
			result := reconciler.getControllerPluginReplicas(log, specReplicas)
			Expect(result).To(Equal(specReplicas))
			Expect(*result).To(Equal(int32(3)))
		})
	})
})
