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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	csiv1a1 "github.com/ceph/ceph-csi-operator/api/v1alpha1"
)

//+kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofiles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofiles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofiles/finalizers,verbs=update
//+kubebuilder:rbac:groups=csi.ceph.io,resources=cephconnections,verbs=get;list;watch;update

// ClientProfileReconciler reconciles a ClientProfile object
type ClientProfileReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// A local reconcile object tied to a single reconcile iteration
type ClientProfileReconcile struct {
	ClientProfileReconciler

	ctx           context.Context
	log           logr.Logger
	clientProfile csiv1a1.ClientProfile
	cephConn      csiv1a1.CephConnection
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClientProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Filter update events based on metadata.generation changes, will filter events
	// for non-spec changes on most resource types.
	genChangedPredicate := predicate.GenerationChangedPredicate{}

	return ctrl.NewControllerManagedBy(mgr).
		For(&csiv1a1.ClientProfile{}).
		Owns(
			&csiv1a1.CephConnection{},
			builder.MatchEveryOwner,
			builder.WithPredicates(genChangedPredicate),
		).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClientProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Starting reconcile iteration for ClientProfile", "req", req)

	reconcileHandler := ClientProfileReconcile{}
	reconcileHandler.ClientProfileReconciler = *r
	reconcileHandler.ctx = ctx
	reconcileHandler.log = log
	reconcileHandler.clientProfile.Name = req.Name
	reconcileHandler.clientProfile.Namespace = req.Namespace

	err := reconcileHandler.reconcile()
	if err != nil {
		log.Error(err, "ClientProfile reconciliation failed")
	} else {
		log.Info("ClientProfile reconciliation completed successfully")
	}
	return ctrl.Result{}, err
}

func (r *ClientProfileReconcile) reconcile() error {
	// Load the ClientProfile
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(&r.clientProfile), &r.clientProfile); err != nil {
		r.log.Error(err, "Failed loading ClientProfile")
		return err
	}

	// Validate a pointer to a ceph connection
	if r.clientProfile.Spec.CephConnectionRef.Name == "" {
		err := fmt.Errorf("validation error")
		r.log.Error(err, "Invalid ClientProfile, missing .spec.cephConnectionRef.name")
		return err
	}

	// Load the ceph connection
	r.cephConn.Name = r.clientProfile.Spec.CephConnectionRef.Name
	r.cephConn.Namespace = r.clientProfile.Namespace
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(&r.cephConn), &r.cephConn); err != nil {
		r.log.Error(err, "Failed loading CephConnection")
		return err
	}

	// Ensure the CephConnection has an owner reference (not controller reference)
	// for the current reconciled ClientProfile
	connHasOwnerRef := false
	for i := range r.cephConn.OwnerReferences {
		ownerRef := &r.cephConn.OwnerReferences[i]
		if ownerRef.UID == r.clientProfile.UID {
			connHasOwnerRef = true
			break
		}
	}
	if !connHasOwnerRef {
		if err := ctrlutil.SetOwnerReference(&r.clientProfile, &r.cephConn, r.Scheme); err != nil {
			r.log.Error(err, "Failed adding an owner reference on CephConnection")
			return err
		}
		r.log.Info("Owner reference missing on CephConnection, adding")
		if err := r.Update(r.ctx, &r.cephConn); err != nil {
			r.log.Error(err, "Failed adding an owner reference to CephConnection")
			return err
		}
	}

	return nil
}
