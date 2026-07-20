/*
Copyright 2026.

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
	"sort"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	csiv1 "github.com/ceph/ceph-csi-operator/api/v1"
	"github.com/ceph/ceph-csi-operator/internal/utils"
)

// +kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofilereplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofilereplications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofilereplications/finalizers,verbs=update
// +kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofiles,verbs=get;list;watch

const (
	clientProfileIndexKey = "index:spec.localClientProfile"
)

// ClientProfileReplicationReconciler reconciles a ClientProfileReplication object
type ClientProfileReplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// A local reconcile object tied to a single reconcile iteration
type ClientProfileReplicationReconcile struct {
	ClientProfileReplicationReconciler

	ctx                      context.Context
	log                      logr.Logger
	clientProfileReplication csiv1.ClientProfileReplication
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClientProfileReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create field index for efficient lookup by clientProfileIndex
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&csiv1.ClientProfileReplication{},
		clientProfileIndexKey,
		func(obj client.Object) []string {
			cpr := obj.(*csiv1.ClientProfileReplication)
			if cpr.Spec.LocalClientProfile != "" {
				return []string{cpr.Spec.LocalClientProfile}
			}
			return nil
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&csiv1.ClientProfileReplication{}).
		// Watch ClientProfile CRs to trigger reconciliation when they are created/deleted
		Watches(
			&csiv1.ClientProfile{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				clientProfile := obj.(*csiv1.ClientProfile)

				cprList := &csiv1.ClientProfileReplicationList{}
				if err := r.List(ctx, cprList,
					client.InNamespace(clientProfile.Namespace),
					client.MatchingFields{clientProfileIndexKey: clientProfile.Name}); err != nil {
					return []reconcile.Request{}
				}

				requests := make([]reconcile.Request, len(cprList.Items))
				for i, item := range cprList.Items {
					requests[i] = reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						},
					}
				}
				return requests
			}),
			builder.WithPredicates(
				utils.EventTypePredicate(true, false, true, false),
			),
		).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClientProfileReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Starting reconcile iteration for ClientProfileReplication", "req", req)

	reconcileHandler := ClientProfileReplicationReconcile{}
	reconcileHandler.ClientProfileReplicationReconciler = *r
	reconcileHandler.ctx = ctx
	reconcileHandler.log = log
	reconcileHandler.clientProfileReplication.Name = req.Name
	reconcileHandler.clientProfileReplication.Namespace = req.Namespace

	err := reconcileHandler.reconcile()
	if err != nil {
		log.Error(err, "ClientProfileReplication reconciliation failed")
	} else {
		log.Info("ClientProfileReplication reconciliation completed successfully")
	}

	return ctrl.Result{}, err
}

func (r *ClientProfileReplicationReconcile) reconcile() error {

	// Fetch the ClientProfileReplication CR
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(&r.clientProfileReplication), &r.clientProfileReplication); err != nil {
		if errors.IsNotFound(err) {
			r.log.Info("ClientProfileReplication not found, ignoring")
			return nil
		}
		r.log.Error(err, "failed to get ClientProfileReplication")
		return err
	}

	reconcileErr := r.reconcilePhases()

	statusErr := r.Status().Update(r.ctx, &r.clientProfileReplication)
	if statusErr != nil {
		r.log.Error(statusErr, "Failed to update ClientProfileReplication status.")
	}
	if reconcileErr != nil {
		return reconcileErr
	} else if statusErr != nil {
		return statusErr
	}
	return nil
}

func (r *ClientProfileReplicationReconcile) reconcilePhases() error {
	r.clientProfileReplication.Status.Phase = csiv1.ClientProfileReplicationPhasePending
	// Validate that the referenced ClientProfile exists
	clientProfile := &csiv1.ClientProfile{}
	clientProfile.Name = r.clientProfileReplication.Spec.LocalClientProfile
	clientProfile.Namespace = r.clientProfileReplication.Namespace
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(clientProfile), clientProfile); err != nil {
		if errors.IsNotFound(err) {
			// ClientProfile not found - reject this CR
			r.log.Info("referenced ClientProfile not found, rejecting", "clientProfile", clientProfile.Name)
			r.clientProfileReplication.Status.Phase = csiv1.ClientProfileReplicationPhaseRejected
			r.clientProfileReplication.Status.Message = fmt.Sprintf("rejected: ClientProfile '%s' not found", clientProfile.Name)
			return nil
		}
		r.log.Error(err, "failed to get ClientProfile")
		return err
	}

	// Look up all CRs referencing the same localClientProfile
	cprList := &csiv1.ClientProfileReplicationList{}
	if err := r.List(
		r.ctx,
		cprList,
		client.InNamespace(r.clientProfileReplication.Namespace),
		client.MatchingFields{clientProfileIndexKey: clientProfile.Name},
	); err != nil {
		r.log.Error(err, "failed to list ClientProfileReplication CRs by localClientProfile")
		return err
	}

	// Step 3: Conflict detection - oldest CR wins
	// Sort by creation timestamp (oldest first)
	sort.Slice(cprList.Items, func(i, j int) bool {
		cprI := cprList.Items[i]
		cprJ := cprList.Items[j]

		if !cprI.CreationTimestamp.Equal(&cprJ.CreationTimestamp) {
			return cprI.CreationTimestamp.Before(&cprJ.CreationTimestamp)
		}

		// If timestamps are identical, oldest/first is determined by resource name
		return cprI.Name < cprJ.Name
	})

	// The oldest CR is the winner
	winner := &cprList.Items[0]

	// Step 4: Update status based on whether this CR is the winner
	if r.clientProfileReplication.Name == winner.Name && r.clientProfileReplication.Namespace == winner.Namespace {
		r.log.Info("this CR is the winner, marking as Ready")
		r.clientProfileReplication.Status.Phase = csiv1.ClientProfileReplicationPhaseReady
		r.clientProfileReplication.Status.Message = "accepted"
	} else {
		// This CR is not the winner - mark as Rejected
		r.log.Info("more than one clientProfileReplication exist, marking as Rejected", "existing", winner.Name)
		r.clientProfileReplication.Status.Phase = csiv1.ClientProfileReplicationPhaseRejected
		r.clientProfileReplication.Status.Message = fmt.Sprintf(
			"rejected: another ClientProfileReplication '%s' is already active for localClientProfile '%s'",
			winner.Name,
			clientProfile.Name,
		)
	}

	return nil
}
