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

var _ = Describe("ClientProfile Controller with Fake Client", func() {
	var (
		ctx                context.Context
		fakeClient         client.Client
		reconciler         *ClientProfileReconciler
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
			WithStatusSubresource(&csiv1.ClientProfile{}).
			WithIndex(&csiv1.ClientProfileReplication{}, clientProfileIndexKey, func(obj client.Object) []string {
				cpr := obj.(*csiv1.ClientProfileReplication)
				if cpr.Spec.LocalClientProfile != "" {
					return []string{cpr.Spec.LocalClientProfile}
				}
				return nil
			}).
			Build()

		reconciler = &ClientProfileReconciler{
			Client: fakeClient,
			Scheme: testScheme,
		}
	})

	Context("When no ClientProfileReplication exists", func() {
		It("should reconcile successfully without replication destination", func() {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testClientProfile.Name,
					Namespace: testClientProfile.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify status
			updated := &csiv1.ClientProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(testClientProfile), updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(csiv1.ClientProfilePhaseReady))
		})
	})

	Context("When a Ready ClientProfileReplication exists", func() {
		It("should reconcile successfully with replication destination", func() {
			// Create a Ready ClientProfileReplication
			cpr := &csiv1.ClientProfileReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication",
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
				Status: csiv1.ClientProfileReplicationStatus{
					Phase:   csiv1.ClientProfileReplicationPhaseReady,
					Message: "accepted",
				},
			}
			Expect(fakeClient.Create(ctx, cpr)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testClientProfile.Name,
					Namespace: testClientProfile.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify status
			updated := &csiv1.ClientProfile{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(testClientProfile), updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(csiv1.ClientProfilePhaseReady))
		})
	})

	Context("When multiple Ready ClientProfileReplication CRs exist", func() {
		It("should fail reconciliation", func() {
			// Create two Ready ClientProfileReplication CRs
			cpr1 := &csiv1.ClientProfileReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication-1",
					Namespace: "default",
				},
				Spec: csiv1.ClientProfileReplicationSpec{
					LocalClientProfile:  testClientProfile.Name,
					RemoteClientProfile: "remote-profile-1",
				},
				Status: csiv1.ClientProfileReplicationStatus{
					Phase:   csiv1.ClientProfileReplicationPhaseReady,
					Message: "accepted",
				},
			}
			Expect(fakeClient.Create(ctx, cpr1)).To(Succeed())

			cpr2 := &csiv1.ClientProfileReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication-2",
					Namespace: "default",
				},
				Spec: csiv1.ClientProfileReplicationSpec{
					LocalClientProfile:  testClientProfile.Name,
					RemoteClientProfile: "remote-profile-2",
				},
				Status: csiv1.ClientProfileReplicationStatus{
					Phase:   csiv1.ClientProfileReplicationPhaseReady,
					Message: "accepted",
				},
			}
			Expect(fakeClient.Create(ctx, cpr2)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testClientProfile.Name,
					Namespace: testClientProfile.Namespace,
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("multiple Ready ClientProfileReplication CRs found"))
		})
	})

	Context("When ClientProfile is being deleted with referencing ClientProfileReplication", func() {
		It("should fail to delete", func() {
			// Create objects with ClientProfile marked for deletion
			now := metav1.Now()
			deletingProfile := &csiv1.ClientProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "deleting-profile",
					Namespace:         "default",
					DeletionTimestamp: &now,
					Finalizers:        []string{cleanupFinalizer},
				},
				Spec: csiv1.ClientProfileSpec{
					CephConnectionRef: corev1.LocalObjectReference{
						Name: testCephConnection.Name,
					},
				},
			}

			cpr := &csiv1.ClientProfileReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication",
					Namespace: "default",
				},
				Spec: csiv1.ClientProfileReplicationSpec{
					LocalClientProfile:  deletingProfile.Name,
					RemoteClientProfile: "remote-profile",
				},
			}

			// Create a new fake client with these objects
			testFakeClient := fake.NewClientBuilder().
				WithScheme(testScheme).
				WithObjects(testCephConnection, deletingProfile, cpr).
				WithStatusSubresource(&csiv1.ClientProfile{}).
				WithIndex(&csiv1.ClientProfileReplication{}, clientProfileIndexKey, func(obj client.Object) []string {
					cprObj := obj.(*csiv1.ClientProfileReplication)
					if cprObj.Spec.LocalClientProfile != "" {
						return []string{cprObj.Spec.LocalClientProfile}
					}
					return nil
				}).
				Build()

			testReconciler := &ClientProfileReconciler{
				Client: testFakeClient,
				Scheme: testScheme,
			}

			_, err := testReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      deletingProfile.Name,
					Namespace: deletingProfile.Namespace,
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ClientProfileReplication CRs still reference this profile"))
		})
	})
})
