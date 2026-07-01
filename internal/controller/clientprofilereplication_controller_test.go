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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	csiv1 "github.com/ceph/ceph-csi-operator/api/v1"
)

var _ = Describe("ClientProfileReplication Controller with Fake Client", func() {
	var (
		ctx                context.Context
		fakeClient         client.Client
		reconciler         *ClientProfileReplicationReconciler
		testScheme         *runtime.Scheme
		testClientProfile  *csiv1.ClientProfile
		testCephConnection *csiv1.CephConnection
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create scheme
		testScheme = runtime.NewScheme()
		Expect(csiv1.AddToScheme(testScheme)).To(Succeed())
		Expect(scheme.AddToScheme(testScheme)).To(Succeed())

		// Create test objects
		testCephConnection = &csiv1.CephConnection{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ceph-connection",
				Namespace: "default",
			},
			Spec: csiv1.CephConnectionSpec{
				Monitors: []string{"mon1:6789", "mon2:6789"},
			},
		}

		testClientProfile = &csiv1.ClientProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-client-profile",
				Namespace: "default",
			},
			Spec: csiv1.ClientProfileSpec{
				CephConnectionRef: corev1.LocalObjectReference{
					Name: testCephConnection.Name,
				},
			},
		}

		// Create fake client with field index
		fakeClient = fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(testCephConnection, testClientProfile).
			WithStatusSubresource(&csiv1.ClientProfileReplication{}).
			WithIndex(&csiv1.ClientProfileReplication{}, clientProfileIndexKey, func(obj client.Object) []string {
				cpr := obj.(*csiv1.ClientProfileReplication)
				if cpr.Spec.LocalClientProfile != "" {
					return []string{cpr.Spec.LocalClientProfile}
				}
				return nil
			}).
			Build()

		reconciler = &ClientProfileReplicationReconciler{
			Client: fakeClient,
			Scheme: testScheme,
		}
	})

	Context("When ClientProfile does not exist", func() {
		It("should reject the ClientProfileReplication CR", func() {
			cpr := &csiv1.ClientProfileReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cpr-no-profile",
					Namespace: "default",
				},
				Spec: csiv1.ClientProfileReplicationSpec{
					LocalClientProfile:  "nonexistent-profile",
					RemoteClientProfile: "remote-profile",
				},
			}
			Expect(fakeClient.Create(ctx, cpr)).To(Succeed())

			// Reconcile
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      cpr.Name,
					Namespace: cpr.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify status
			updated := &csiv1.ClientProfileReplication{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{
				Name:      cpr.Name,
				Namespace: cpr.Namespace,
			}, updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(csiv1.ClientProfileReplicationPhaseRejected))
			Expect(updated.Status.Message).To(ContainSubstring("ClientProfile 'nonexistent-profile' not found"))
		})
	})

	Context("When single ClientProfileReplication exists", func() {
		It("should mark it as Ready", func() {
			cpr := &csiv1.ClientProfileReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cpr-single",
					Namespace: "default",
				},
				Spec: csiv1.ClientProfileReplicationSpec{
					LocalClientProfile:  testClientProfile.Name,
					RemoteClientProfile: "remote-profile",
					RBD: &csiv1.RBDReplicationSpec{
						PoolMapping: []csiv1.PoolMappingSpec{
							{Name: "rbd", RemoteID: "5"},
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, cpr)).To(Succeed())

			// Reconcile
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      cpr.Name,
					Namespace: cpr.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify status
			updated := &csiv1.ClientProfileReplication{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{
				Name:      cpr.Name,
				Namespace: cpr.Namespace,
			}, updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(csiv1.ClientProfileReplicationPhaseReady))
			Expect(updated.Status.Message).To(Equal("accepted"))
		})
	})

	Context("When multiple ClientProfileReplication CRs exist", func() {
		It("should mark oldest as Ready and others as Rejected", func() {
			// Pre-create objects with explicit timestamps
			oldTime := metav1.NewTime(time.Now().Add(-10 * time.Minute))
			newTime := metav1.Now()

			cpr1 := &csiv1.ClientProfileReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cpr-oldest",
					Namespace:         "default",
					CreationTimestamp: oldTime,
				},
				Spec: csiv1.ClientProfileReplicationSpec{
					LocalClientProfile:  testClientProfile.Name,
					RemoteClientProfile: "remote-profile-1",
				},
			}

			cpr2 := &csiv1.ClientProfileReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cpr-newer",
					Namespace:         "default",
					CreationTimestamp: newTime,
				},
				Spec: csiv1.ClientProfileReplicationSpec{
					LocalClientProfile:  testClientProfile.Name,
					RemoteClientProfile: "remote-profile-2",
				},
			}

			// Create a new fake client with both objects pre-populated
			testFakeClient := fake.NewClientBuilder().
				WithScheme(testScheme).
				WithObjects(testCephConnection, testClientProfile, cpr1, cpr2).
				WithStatusSubresource(&csiv1.ClientProfileReplication{}).
				WithIndex(&csiv1.ClientProfileReplication{}, clientProfileIndexKey, func(obj client.Object) []string {
					cpr := obj.(*csiv1.ClientProfileReplication)
					if cpr.Spec.LocalClientProfile != "" {
						return []string{cpr.Spec.LocalClientProfile}
					}
					return nil
				}).
				Build()

			testReconciler := &ClientProfileReplicationReconciler{
				Client: testFakeClient,
				Scheme: testScheme,
			}

			// Reconcile both
			_, err := testReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: cpr1.Name, Namespace: cpr1.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = testReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: cpr2.Name, Namespace: cpr2.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify oldest is Ready
			updated1 := &csiv1.ClientProfileReplication{}
			Expect(testFakeClient.Get(ctx, types.NamespacedName{Name: cpr1.Name, Namespace: cpr1.Namespace}, updated1)).To(Succeed())
			Expect(updated1.Status.Phase).To(Equal(csiv1.ClientProfileReplicationPhaseReady))

			// Verify newer is Rejected
			updated2 := &csiv1.ClientProfileReplication{}
			Expect(testFakeClient.Get(ctx, types.NamespacedName{Name: cpr2.Name, Namespace: cpr2.Namespace}, updated2)).To(Succeed())
			Expect(updated2.Status.Phase).To(Equal(csiv1.ClientProfileReplicationPhaseRejected))
			Expect(updated2.Status.Message).To(ContainSubstring(cpr1.Name))
		})
	})
})
