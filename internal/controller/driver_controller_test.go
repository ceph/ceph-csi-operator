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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	csiv1 "github.com/ceph/ceph-csi-operator/api/v1"
	"github.com/ceph/ceph-csi-operator/internal/utils"
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

	Context("PodMonitor reconciliation", func() {
		const resourceName = "podmonitor.rbd.csi.ceph.com"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		podMonitorNamespacedName := types.NamespacedName{
			Name:      resourceName + "-podmonitor",
			Namespace: "default",
		}
		driver := &csiv1.Driver{}

		newDriver := func(podMonitorEnabled bool) *csiv1.Driver {
			return &csiv1.Driver{
				ObjectMeta: metav1.ObjectMeta{
					Name:      typeNamespacedName.Name,
					Namespace: typeNamespacedName.Namespace,
				},
				Spec: csiv1.DriverSpec{
					Liveness: &csiv1.LivenessSpec{MetricsPort: 9090},
					PodMonitor: &csiv1.PodMonitorSpec{
						Enabled:     ptr.To(podMonitorEnabled),
						Labels:      map[string]string{"monitoring": "ceph-csi"},
						Annotations: map[string]string{"owner": "ceph-csi-operator"},
						Interval:    "30s",
					},
				},
			}
		}

		reconcileDriver := func() {
			By("reconciling the Driver")
			controllerReconciler := &DriverReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(
				ctx,
				reconcile.Request{NamespacedName: typeNamespacedName},
			)
			Expect(err).NotTo(HaveOccurred())
		}

		deletePodMonitor := func() {
			podMonitor := &monitoringv1.PodMonitor{}
			err := k8sClient.Get(ctx, podMonitorNamespacedName, podMonitor)
			if err == nil {
				Expect(k8sClient.Delete(ctx, podMonitor)).To(Succeed())
			}
		}

		BeforeEach(func() {
			By("creating a Driver with liveness and PodMonitor enabled")
			driver = newDriver(true)
			Expect(k8sClient.Create(ctx, driver)).To(Succeed())
		})

		AfterEach(func() {
			By("cleaning up the Driver and the PodMonitor")
			driverObj := &csiv1.Driver{}
			if err := k8sClient.Get(ctx, typeNamespacedName, driverObj); err == nil {
				Expect(k8sClient.Delete(ctx, driverObj)).To(Succeed())
			}
			deletePodMonitor()
		})

		It("should create a PodMonitor selecting the driver's pods and metrics port", func() {
			reconcileDriver()

			podMonitor := &monitoringv1.PodMonitor{}
			Expect(k8sClient.Get(ctx, podMonitorNamespacedName, podMonitor)).To(Succeed())

			By("verifying the PodMonitor is owned by the Driver")
			Expect(podMonitor.OwnerReferences).To(HaveLen(1))
			Expect(podMonitor.OwnerReferences[0].Name).To(Equal(typeNamespacedName.Name))

			By("verifying the PodMonitor selects the controller plugin and node plugin pods")
			Expect(podMonitor.Spec.Selector.MatchExpressions).To(ConsistOf(
				metav1.LabelSelectorRequirement{
					Key:      "app",
					Operator: metav1.LabelSelectorOpIn,
					Values: []string{
						resourceName + "-ctrlplugin",
						resourceName + "-nodeplugin",
					},
				},
			))

			By("verifying the PodMonitor scrapes the liveness sidecar's metrics port")
			Expect(podMonitor.Spec.PodMetricsEndpoints).To(HaveLen(1))
			endpoint := podMonitor.Spec.PodMetricsEndpoints[0]
			Expect(ptr.Deref(endpoint.Port, "")).To(Equal(utils.LivenessMetricsContainerPortName))
			Expect(endpoint.Path).To(Equal(utils.LivenessMetricsPath))
			Expect(string(endpoint.Interval)).To(Equal("30s"))

			By("verifying the user defined labels and annotations are propagated")
			Expect(podMonitor.Labels).To(HaveKeyWithValue("monitoring", "ceph-csi"))
			Expect(podMonitor.Annotations).To(HaveKeyWithValue("owner", "ceph-csi-operator"))
		})

		It("should remove the PodMonitor when the feature is disabled", func() {
			reconcileDriver()

			podMonitor := &monitoringv1.PodMonitor{}
			Expect(k8sClient.Get(ctx, podMonitorNamespacedName, podMonitor)).To(Succeed())

			By("disabling the PodMonitor on the Driver")
			Expect(k8sClient.Get(ctx, typeNamespacedName, driver)).To(Succeed())
			driver.Spec.PodMonitor.Enabled = ptr.To(false)
			Expect(k8sClient.Update(ctx, driver)).To(Succeed())

			reconcileDriver()

			By("verifying the PodMonitor was removed")
			err := k8sClient.Get(ctx, podMonitorNamespacedName, &monitoringv1.PodMonitor{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should not create a PodMonitor when the driver's liveness is not configured", func() {
			By("replacing the Driver with one without liveness configured")
			Expect(k8sClient.Delete(ctx, driver)).To(Succeed())
			driver = newDriver(true)
			driver.Spec.Liveness = nil
			Expect(k8sClient.Create(ctx, driver)).To(Succeed())

			reconcileDriver()

			By("verifying no PodMonitor was created")
			err := k8sClient.Get(ctx, podMonitorNamespacedName, &monitoringv1.PodMonitor{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})
