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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	csiv1 "github.com/ceph/ceph-csi-operator/api/v1"
	"github.com/ceph/ceph-csi-operator/internal/utils"
)

//+kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofilemappings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofilemappings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csi.ceph.io,resources=clientprofilemappings/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;delete

// ClientProfileMappingReconciler reconciles a ClientProfileMapping object
type ClientProfileMappingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// A local reconcile object tied to a single reconcile iteration
type ClientProfileMappingReconcile struct {
	ClientProfileMappingReconciler

	ctx                      context.Context
	log                      logr.Logger
	req                      ctrl.Request
	clientProfileMappingList csiv1.ClientProfileMappingList
}

// csiClusterMappingRecord represents the structure to serialize a csi mapping
// record in Ceph CSI's config
type csiClusterMappingRecord struct {
	ClusterIdMapping map[string]string   `json:"clusterIdMapping,omitempty"`
	RbdPoolIdMapping []map[string]string `json:"RBDPoolIDMapping,omitempty"`
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClientProfileMappingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	genChangedPredicate := predicate.GenerationChangedPredicate{}

	return ctrl.NewControllerManagedBy(mgr).
		For(&csiv1.ClientProfileMapping{}).
		// TODO: This watch is probalematic, it will trigger a reconcile for each
		// mapping resource that exists in the configmap namespace whenever the configmap
		// is changed. This is unnececary as this reconile build its desired state based
		// on all mapping resources within a single reconcile
		Owns(
			&corev1.ConfigMap{},
			builder.MatchEveryOwner,
			builder.WithPredicates(
				utils.NamePredicate(utils.CsiConfigVolume.Name),
				genChangedPredicate,
			),
		).
		Complete(r)
}

func (r *ClientProfileMappingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Starting reconcile iteration for ClientProfileMapping", "req", req)

	reconcileHandler := ClientProfileMappingReconcile{}
	reconcileHandler.ClientProfileMappingReconciler = *r
	reconcileHandler.ctx = ctx
	reconcileHandler.log = log
	reconcileHandler.req = req

	err := reconcileHandler.reconcile()
	if err != nil {
		log.Error(err, "ClientProfileMapping reconciliation failed")
	} else {
		log.Info("ClientProfileMapping reconciliation completed successfully")
	}
	return ctrl.Result{}, err
}

func (r *ClientProfileMappingReconcile) reconcile() error {
	// This controller behave a differently then other controller. Because of the lack of uniqueness of the mapping
	// information between 2 or more mapping resources, there is no way to update the csi mapping information without
	// building it from the ground up on every reconcile. To do that we ignore the specific of the subject resource
	// and instead load all mapping resources within the namespace.
	if err := r.List(r.ctx, &r.clientProfileMappingList, client.InNamespace(r.req.Namespace)); err != nil {
		r.log.Error(err, "Failed listing ClientProfileMapping CRs in namespace", "namespace", r.req.Namespace)
		return err
	}

	if err := r.reconcileCephCsiBlockPoolMapping(); err != nil {
		return err
	}

	return nil
}

func (r *ClientProfileMappingReconcile) reconcileCephCsiBlockPoolMapping() error {
	csiConfigMap := corev1.ConfigMap{}
	csiConfigMap.Name = utils.CsiConfigVolume.Name
	csiConfigMap.Namespace = r.req.Namespace

	log := r.log.WithValues("csiConfigMapName", csiConfigMap.Name)
	log.Info("Reconciling Ceph CSI Cluster mapping")

	_, err := ctrlutil.CreateOrUpdate(r.ctx, r.Client, &csiConfigMap, func() error {
		var owner *csiv1.ClientProfileMapping
		for i := range r.clientProfileMappingList.Items {
			item := &r.clientProfileMappingList.Items[i]
			if item.Name == r.req.Name && item.Namespace == r.req.Namespace {
				owner = item
				break
			}

		}
		if owner != nil {
			if _, err := utils.ToggleOwnerReference(true, &csiConfigMap, owner, r.Scheme); err != nil {
				log.Error(err, "Failed toggling owner reference for Ceph CSI config map")
				return err
			}
		}

		type mappingKey [2]string
		type duplicationKey [4]string
		indexByPair := map[mappingKey]int{}
		csiClusterMappingsList := []csiClusterMappingRecord{}
		alreadySeen := map[duplicationKey]bool{}

		// Scan every loaded profile mapping CR, for each scan all mappings records.
		for i := range r.clientProfileMappingList.Items {
			spec := &r.clientProfileMappingList.Items[i].Spec
			for j := range spec.Mappings {
				mapping := &spec.Mappings[j]

				// Create a local+remote key
				key := mappingKey{mapping.LocalClientProfile, mapping.RemoteClientProfile}

				// Check if we already encountered the local+remote pair. If we didn't,
				// append a new record at the end, to the csi mapping config
				index, ok := indexByPair[key]
				if !ok {
					index = len(csiClusterMappingsList)
					indexByPair[key] = index
					csiClusterMappingsList = append(
						csiClusterMappingsList,
						csiClusterMappingRecord{
							ClusterIdMapping: map[string]string{
								mapping.LocalClientProfile: mapping.RemoteClientProfile,
							},
							RbdPoolIdMapping: []map[string]string{},
						},
					)
				}

				// Transform and copy mapping information from ClientProfileMapping types
				// into the csi mapping types
				rbdPoolIdMapping := csiClusterMappingsList[index].RbdPoolIdMapping
				for _, pair := range mapping.BlockPoolIdMapping {
					dupKey := duplicationKey{key[0], key[1], pair[0], pair[1]}

					// Skip adding identical items
					if !alreadySeen[dupKey] {
						rbdPoolIdMapping = append(rbdPoolIdMapping, map[string]string{pair[0]: pair[1]})
						alreadySeen[dupKey] = true
					}
				}
				csiClusterMappingsList[index].RbdPoolIdMapping = rbdPoolIdMapping
			}
		}

		if bytes, err := json.Marshal(csiClusterMappingsList); err != nil {
			log.Error(err, "Failed to serialize cluster mappings list")
			return err
		} else {
			if csiConfigMap.Data == nil {
				csiConfigMap.Data = map[string]string{}
			}
			csiConfigMap.Data[utils.CsiConfigMapMappingKey] = string(bytes)
			return nil
		}
	})

	return err
}
