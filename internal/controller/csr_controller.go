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
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"

	certv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//+kubebuilder:rbac:groups=csi.ceph.io,resources=drivers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csi.ceph.io,resources=drivers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csi.ceph.io,resources=drivers/finalizers,verbs=update
//+kubebuilder:rbac:groups=csi.ceph.io,resources=operatorconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

type CertsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func filterCsiCertificateSigningRequests() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		labels := object.GetLabels()
		return labels != nil && labels["managed-by"] == "ceph-csi-operator"
	})
}

func (r *CertsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&certv1.CertificateSigningRequest{}).WithEventFilter(filterCsiCertificateSigningRequests()).Complete(r)
}

func (c *CertsReconciler) retrieveSignedCertificate(csr *certv1.CertificateSigningRequest) (ctrl.Result, error) {
	if csr.Status.Certificate == nil {
		// Retry later if the certificate is not ready yet
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}
	return ctrl.Result{}, nil
}

func (r *CertsReconciler) Reconcile(ctx context.Context, request ctrl.Request) (reconcile.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Starting reconcile for Certificate signing requests", "req", request)

	csr := &certv1.CertificateSigningRequest{}
	if err := r.Get(ctx, request.NamespacedName, csr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if isCSRApproved(csr) {
		return r.retrieveSignedCertificate(csr)
	}

	// Validate CSR contents to ensure it meets the requirements
	if err := validateCSR(csr); err != nil {
		// Log the error, or update the status to indicate failure
		return ctrl.Result{}, err
	}

	// Approve the CSR
	csr.Status.Conditions = append(csr.Status.Conditions, certv1.CertificateSigningRequestCondition{
		Type:           certv1.CertificateApproved,
		Status:         v1.ConditionTrue,
		Reason:         "AutoApproved",
		Message:        "CSR auto-approved by Ceph CSI CSR Controller",
		LastUpdateTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, csr); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func isCSRApproved(csr *certv1.CertificateSigningRequest) bool {
	for _, condition := range csr.Status.Conditions {
		if condition.Type == certv1.CertificateApproved {
			return true
		}
	}
	return false
}

// validateCSR validates CSR fields for the required certificate attributes
func validateCSR(csr *certv1.CertificateSigningRequest) error {
	csrBytes, _ := pem.Decode(csr.Spec.Request)
	if csrBytes == nil {
		return errors.New("failed to parse CSR PEM")
	}
	certRequest, err := x509.ParseCertificateRequest(csrBytes.Bytes)
	if err != nil {
		return err
	}

	// Validate required fields (e.g., organization, common name, etc.)
	if len(certRequest.Subject.Organization) == 0 || certRequest.Subject.CommonName == "" {
		return errors.New("CSR is missing required fields")
	}

	return nil
}
