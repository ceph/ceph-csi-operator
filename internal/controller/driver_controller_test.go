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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
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
			Namespace: defaultNamespace, // TODO(user):Modify as needed
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

var _ = Describe("imageSetConfigMapPredicate", func() {
	const (
		pluginKey = "plugin"
		pluginV1  = "plugin:v1"
		pluginV2  = "plugin:v2"
	)
	imageSetPredicate := imageSetConfigMapPredicate()
	configMapWithData := func(images map[string]string) *corev1.ConfigMap {
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "image-set", Namespace: "default"},
			Data:       images,
		}
	}

	It("should accept an update that changes the data", func() {
		Expect(imageSetPredicate.Update(event.UpdateEvent{
			ObjectOld: configMapWithData(map[string]string{pluginKey: pluginV1}),
			ObjectNew: configMapWithData(map[string]string{pluginKey: pluginV2}),
		})).To(BeTrue())
	})

	It("should filter an update that does not change the data", func() {
		Expect(imageSetPredicate.Update(event.UpdateEvent{
			ObjectOld: configMapWithData(map[string]string{pluginKey: pluginV1}),
			ObjectNew: configMapWithData(map[string]string{pluginKey: pluginV1}),
		})).To(BeFalse())
	})

	It("should accept create and delete events", func() {
		Expect(imageSetPredicate.Create(event.CreateEvent{
			Object: configMapWithData(map[string]string{pluginKey: pluginV1}),
		})).To(BeTrue())
		Expect(imageSetPredicate.Delete(event.DeleteEvent{
			Object: configMapWithData(map[string]string{pluginKey: pluginV1}),
		})).To(BeTrue())
	})

	It("should filter generic events", func() {
		Expect(imageSetPredicate.Generic(event.GenericEvent{
			Object: configMapWithData(map[string]string{pluginKey: pluginV1}),
		})).To(BeFalse())
	})
})

var _ = Describe("imageSetConfigMapRequests", func() {
	const (
		operatorImageSet  = "test-operator-image-set"
		driverImageSet    = "test-driver-image-set"
		otherNamespace    = "test-other-namespace"
		referencingDriver = "test-image-set-referencing.rbd.csi.ceph.com"
		independentDriver = "test-image-set-independent.rbd.csi.ceph.com"
	)
	var (
		ctx        = context.Background()
		fakeClient client.Client
	)

	newDriver := func(name, namespace, imageSet string) *csiv1.Driver {
		driver := &csiv1.Driver{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		}
		if imageSet != "" {
			driver.Spec.ImageSet = &corev1.LocalObjectReference{Name: imageSet}
		}
		return driver
	}

	BeforeEach(func() {
		testScheme := runtime.NewScheme()
		Expect(csiv1.AddToScheme(testScheme)).To(Succeed())

		opConfig := &csiv1.OperatorConfig{
			ObjectMeta: metav1.ObjectMeta{Name: operatorConfigName, Namespace: operatorNamespace},
			Spec: csiv1.OperatorConfigSpec{
				DriverSpecDefaults: &csiv1.DriverSpec{
					ImageSet: &corev1.LocalObjectReference{Name: operatorImageSet},
				},
			},
		}

		By("creating drivers with and without an image set, and one in another namespace")
		fakeClient = fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(
				opConfig,
				newDriver(referencingDriver, defaultNamespace, driverImageSet),
				newDriver(independentDriver, defaultNamespace, ""),
				newDriver(referencingDriver, otherNamespace, driverImageSet),
			).
			WithIndex(&csiv1.Driver{}, driverImageSetIndexKey, driverImageSetIndexFunc).
			Build()
	})

	It("should enqueue all the drivers for the operator level image set", func() {
		requests := imageSetConfigMapRequests(ctx, fakeClient, operatorNamespace, operatorImageSet)
		Expect(requests).To(ConsistOf(
			reconcile.Request{NamespacedName: types.NamespacedName{Name: referencingDriver, Namespace: defaultNamespace}},
			reconcile.Request{NamespacedName: types.NamespacedName{Name: independentDriver, Namespace: defaultNamespace}},
			reconcile.Request{NamespacedName: types.NamespacedName{Name: referencingDriver, Namespace: otherNamespace}},
		))
	})

	It("should enqueue only the drivers in the namespace that reference a per-driver image set", func() {
		requests := imageSetConfigMapRequests(ctx, fakeClient, defaultNamespace, driverImageSet)
		Expect(requests).To(ConsistOf(
			reconcile.Request{NamespacedName: types.NamespacedName{Name: referencingDriver, Namespace: defaultNamespace}},
		))
	})

	It("should not enqueue any driver for a configmap that is not an image set", func() {
		requests := imageSetConfigMapRequests(ctx, fakeClient, defaultNamespace, "unrelated-configmap")
		Expect(requests).To(BeEmpty())
	})

	It("should fall back to the per-driver image sets without an operator config", func() {
		opConfig := &csiv1.OperatorConfig{}
		key := types.NamespacedName{Name: operatorConfigName, Namespace: operatorNamespace}
		Expect(fakeClient.Get(ctx, key, opConfig)).To(Succeed())
		Expect(fakeClient.Delete(ctx, opConfig)).To(Succeed())

		requests := imageSetConfigMapRequests(ctx, fakeClient, defaultNamespace, driverImageSet)
		Expect(requests).To(ConsistOf(
			reconcile.Request{NamespacedName: types.NamespacedName{Name: referencingDriver, Namespace: defaultNamespace}},
		))
	})
})

var _ = Describe("copyNonEmptyValues", func() {
	It("should copy non-empty values and preserve defaults for empty ones", func() {
		const (
			pluginKey      = "plugin"
			provisionerKey = "provisioner"
			attacherKey    = "attacher"
			resizerKey     = "resizer"
		)

		dst := map[string]string{
			pluginKey:      "default-plugin:v1",
			provisionerKey: "default-provisioner:v1",
			attacherKey:    "default-attacher:v1",
		}
		src := map[string]string{
			pluginKey:      "custom-plugin:v2",
			provisionerKey: "",
			resizerKey:     "custom-resizer:v2",
		}

		utils.CopyNonEmptyValues(dst, src)

		Expect(dst).To(Equal(map[string]string{
			pluginKey:      "custom-plugin:v2",
			provisionerKey: "default-provisioner:v1",
			attacherKey:    "default-attacher:v1",
			resizerKey:     "custom-resizer:v2",
		}))
	})
})
