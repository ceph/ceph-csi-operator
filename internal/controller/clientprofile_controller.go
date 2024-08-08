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
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	csiv1a1 "github.com/ceph/ceph-csi-operator/api/v1alpha1"
	"github.com/ceph/ceph-csi-operator/internal/utils"
)

//+kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofiles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofiles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofiles/finalizers,verbs=update
//+kubebuilder:rbac:groups=csi.ceph.io,resources=cephconnections,verbs=get;list;watch;update;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

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
	cleanUp       bool
}

// csiClusterRrcordInfo represent the structure of a serialized csi record
// in Ceph CSI's config, configmap
type csiClusterInfoRecord struct {
	ClusterId string   `json:"clusterID,omitempty"`
	Monitors  []string `json:"monitors,omitempty"`
	CephFs    struct {
		SubvolumeGroup     string `json:"subvolumeGroup,omitempty"`
		KernelMountOptions string `json:"kernelMountOptions"`
		FuseMountOptions   string `json:"fuseMountOptions"`
	} `json:"cephFS,omitempty"`
	Rbd struct {
		RadosNamespace string `json:"radosNamespace,omitempty"`
		MirrorCount    int    `json:"mirrorCount,omitempty"`
	} `json:"rbd,omitempty"`
	Nfs          struct{} `json:"nfs,omitempty"`
	ReadAffinity struct {
		Enabled             bool     `json:"enabled,omitempty"`
		CrushLocationLabels []string `json:"crushLocationLabels,omitempty"`
	} `json:"readAffinity,omitempty"`
}

const (
	cleanupFinalizer = "csi.ceph.com/cleanup"
)

var configMapUpdateLock = sync.Mutex{}

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
		Owns(
			&corev1.ConfigMap{},
			builder.MatchEveryOwner,
			builder.WithPredicates(genChangedPredicate),
		).
		Complete(r)
}

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
	if err := r.loadAndValidate(); err != nil {
		return err
	}

	// Ensure a finalizer on the ClientProfile to allow proper clean up
	if ctrlutil.AddFinalizer(&r.clientProfile, cleanupFinalizer) {
		if err := r.Update(r.ctx, &r.clientProfile); err != nil {
			r.log.Error(err, "Failed to add a cleanup finalizer on ClientProfile")
			return err
		}
	}

	if err := r.reconcileCephConnection(); err != nil {
		return err
	}
	if err := r.reconcileCephCsiClusterInfo(); err != nil {
		return err
	}

	if r.cleanUp {
		ctrlutil.RemoveFinalizer(&r.clientProfile, cleanupFinalizer)
		if err := r.Update(r.ctx, &r.clientProfile); err != nil {
			r.log.Error(err, "Failed to add a cleanup finalizer on config resource")
			return err
		}
	}

	return nil
}

func (r *ClientProfileReconcile) loadAndValidate() error {
	// Load the ClientProfile
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(&r.clientProfile), &r.clientProfile); err != nil {
		r.log.Error(err, "Failed loading ClientProfile")
		return err
	}
	r.cleanUp = r.clientProfile.DeletionTimestamp != nil

	// Validate a pointer to a ceph cluster resource
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
	cephConnHasOwnerRef := false
	for i := range r.cephConn.OwnerReferences {
		ownerRef := &r.cephConn.OwnerReferences[i]
		if ownerRef.UID == r.clientProfile.UID {
			cephConnHasOwnerRef = true
			break
		}
	}
	if !cephConnHasOwnerRef {
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

func (r *ClientProfileReconcile) reconcileCephConnection() error {
	log := r.log.WithValues("cephConnectionName", r.cephConn.Name)
	log.Info("Reconciling CephConnection")

	if needsUpdate, err := utils.ToggleOwnerReference(
		!r.cleanUp,
		&r.cephConn,
		&r.clientProfile,
		r.Scheme,
	); err != nil {
		r.log.Error(err, "Failed to toggle owner reference on CephConnection")
		return err
	} else if needsUpdate {
		if err := r.Update(r.ctx, &r.cephConn); err != nil {
			r.log.Error(err, "Failed to update CephConnection")
			return err
		}
	}

	return nil
}

func (r *ClientProfileReconcile) reconcileCephCsiClusterInfo() error {
	csiConfigMap := corev1.ConfigMap{}
	csiConfigMap.Name = utils.CsiConfigVolume.Name
	csiConfigMap.Namespace = r.clientProfile.Namespace

	log := r.log.WithValues("csiConfigMapName", csiConfigMap.Name)
	log.Info("Reconciling Ceph CSI Cluster Info")

	// Using a lock to serialized the updating of the config map.
	// Although the code will run perfetcly fine without the lock, there will be a higher
	// chance to fail on the create/update operation because another concurrent reconcile loop
	// updated the config map which will result in stale representation and an update failure.
	// The locking strategy will sync all update to the shared config file and will prevent such
	// potential issues without a big impact on preformace as a whole
	configMapUpdateLock.Lock()
	defer configMapUpdateLock.Unlock()

	_, err := ctrlutil.CreateOrUpdate(r.ctx, r.Client, &csiConfigMap, func() error {
		if _, err := utils.ToggleOwnerReference(
			!r.cleanUp,
			&csiConfigMap,
			&r.clientProfile,
			r.Scheme,
		); err != nil {
			log.Error(err, "Failed toggling owner reference for Ceph CSI config map")
			return err
		}

		configsAsJson := csiConfigMap.Data[utils.CsiConfigMapConfigKey]
		clusterInfoList := []*csiClusterInfoRecord{}

		// parse the json serialized list into a go array
		if configsAsJson != "" {
			if err := json.Unmarshal([]byte(configsAsJson), &clusterInfoList); err != nil {
				log.Error(err, "Failed to parse cluster info list under \"config.json\" key")
				return err
			}
		}

		// Locate an existing entry for the same config/cluster name if exists
		index := slices.IndexFunc(clusterInfoList, func(record *csiClusterInfoRecord) bool {
			return record.ClusterId == r.clientProfile.Name
		})

		if !r.cleanUp {
			// Overwrite an existing entry or append a new one
			record := composeCsiClusterInfoRecord(&r.clientProfile, &r.cephConn)
			if index > -1 {
				clusterInfoList[index] = record
			} else {
				clusterInfoList = append(clusterInfoList, record)
			}
		} else if index > -1 {
			// An O(1) unordered in-place delete of a record
			// Will not shrink the capacity of the slice
			length := len(clusterInfoList)
			clusterInfoList[index] = clusterInfoList[length-1]
			clusterInfoList = clusterInfoList[:length-1]
		}

		// Serialize the list and store it back into the config map
		if bytes, err := json.Marshal(clusterInfoList); err != nil {
			log.Error(err, "Failed to serialize cluster info list")
			return err
		} else {
			if csiConfigMap.Data == nil {
				csiConfigMap.Data = map[string]string{}
			}
			csiConfigMap.Data[utils.CsiConfigMapConfigKey] = string(bytes)
			return nil
		}
	})

	return err
}

// ComposeCsiClusterInfoRecord composes the desired csi cluster info record for
// a given ClientProfile and CephConnection specs
func composeCsiClusterInfoRecord(clientProfile *csiv1a1.ClientProfile, cephConn *csiv1a1.CephConnection) *csiClusterInfoRecord {
	record := csiClusterInfoRecord{}
	record.ClusterId = clientProfile.Name
	record.Monitors = cephConn.Spec.Monitors
	if cephFs := clientProfile.Spec.CephFs; cephFs != nil {
		record.CephFs.SubvolumeGroup = cephFs.SubVolumeGroup
		if mountOpt := cephFs.KernelMountOptions; mountOpt != nil {
			record.CephFs.KernelMountOptions = utils.MapToString(mountOpt, "=", ",")
		}
		if mountOpt := cephFs.FuseMountOptions; mountOpt != nil {
			record.CephFs.FuseMountOptions = utils.MapToString(mountOpt, "=", ",")
		}
	}
	if rbd := clientProfile.Spec.Rbd; rbd != nil {
		record.Rbd.RadosNamespace = rbd.RadosNamespace
		record.Rbd.MirrorCount = cephConn.Spec.RbdMirrorDaemonCount
	}
	if readAffinity := cephConn.Spec.ReadAffinity; readAffinity != nil {
		record.ReadAffinity.Enabled = true
		record.ReadAffinity.CrushLocationLabels = readAffinity.CrushLocationLabels
	}
	return &record
}
