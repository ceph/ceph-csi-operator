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
	"errors"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	csiv1a1 "github.com/ceph/ceph-csi-operator/api/v1alpha1"
	"github.com/ceph/ceph-csi-operator/utils"
)

//+kubebuilder:rbac:groups=csi.ceph.io,resources=drivers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csi.ceph.io,resources=drivers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csi.ceph.io,resources=drivers/finalizers,verbs=update
//+kubebuilder:rbac:groups=csi.ceph.io,resources=operatorconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list
//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch;create;update;patch;delete

// Annotation name for ownerref information
const ownerRefAnnotationKey = "csi.ceph.io/ownerref"

// A regexp used to parse driver's prefix and type from the full name
var nameRegExp, _ = regexp.Compile(`^(.*)\.(rbd|cephfs|nfs)\.csi\.ceph\.com$`)

// DriverReconciler reconciles a Driver object
type DriverReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// A local reconcile object tied to a single reconcile iteration
type driverReconcile struct {
	DriverReconciler

	ctx        context.Context
	log        logr.Logger
	driver     csiv1a1.Driver
	driverName string
	driverType string
	images     map[string]string
}

// SetupWithManager sets up the controller with the Manager.
func (r *DriverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Define conditions for an OperatorConfig change that the require queuing of reconciliation
	// request for drivers
	driverDefaultsPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			opConf, ok := e.Object.(*csiv1a1.OperatorConfig)
			return ok && opConf.Spec.DriverSpecDefaults != nil
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldConf, oldOk := e.ObjectOld.(*csiv1a1.OperatorConfig)
			newConf, newOk := e.ObjectNew.(*csiv1a1.OperatorConfig)
			return !oldOk || !newOk ||
				!reflect.DeepEqual(oldConf.Spec.DriverSpecDefaults, newConf.Spec.DriverSpecDefaults)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			opConf, ok := e.Object.(*csiv1a1.OperatorConfig)
			return ok && opConf.Spec.DriverSpecDefaults != nil
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}

	// Enqueue a reconcile request for all existing drivers, used to trigger a reconcile
	// for all drivers whenever the driver default configuration changes
	enqueueAllDrivers := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			driverList := csiv1a1.DriverList{}
			if err := r.List(ctx, &driverList); err != nil {
				return []reconcile.Request{}
			}

			requests := make([]reconcile.Request, len(driverList.Items))
			for i := range driverList.Items {
				requests[i].NamespacedName = client.ObjectKeyFromObject(&driverList.Items[i])
			}
			return requests
		},
	)

	// Enqueue a reconcile request based on an annotation marking a soft ownership
	enqueueFromOwnerRefAnnotation := handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, obj client.Object) []reconcile.Request {
			ownerRef := obj.GetAnnotations()[ownerRefAnnotationKey]
			if ownerRef == "" {
				return nil
			}

			ownerObjKey := client.ObjectKey{}
			if err := json.Unmarshal([]byte(ownerRef), &ownerObjKey); err != nil {
				return nil
			}

			return []reconcile.Request{{
				NamespacedName: ownerObjKey,
			}}
		},
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&csiv1a1.Driver{}).
		Watches(
			&csiv1a1.OperatorConfig{},
			enqueueAllDrivers,
			builder.WithPredicates(driverDefaultsPredicate),
		).
		Watches(
			&storagev1.CSIDriver{},
			enqueueFromOwnerRefAnnotation,
		).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *DriverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reconcileHandler := driverReconcile{}
	reconcileHandler.DriverReconciler = *r
	reconcileHandler.ctx = ctx
	reconcileHandler.log = ctrllog.FromContext(ctx)
	reconcileHandler.driver.Name = req.Name
	reconcileHandler.driver.Namespace = req.Namespace

	return reconcileHandler.reconcile()
}

func (r *driverReconcile) reconcile() (ctrl.Result, error) {
	r.log.Info("Enter Reconcile", "req", client.ObjectKeyFromObject(&r.driver))

	// Load the driver desired state based on driver resource, operator config resource and default values.
	if err := r.LoadAndValidateDesiredState(); err != nil {
		return ctrl.Result{}, err
	}

	// Concurrently reconcile different aspects of the clusters actual state to meet
	// the desired state defined on the driver object
	errChan := utils.RunConcurrently(
		r.reconcileK8sCsiDriver,
		r.reconcilePluginDeamonSet,
		r.reconcileProvisionerDeployment,
		r.reconcileLivnessService,
	)

	// Check if any reconcilatin error where raised during the concurrent execution
	// of the reconciliation steps.
	errList := utils.ChannelToSlice(errChan)
	if err := errors.Join(errList...); err != nil {
		r.log.Error(err, "Reconciliation failed")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *driverReconcile) LoadAndValidateDesiredState() error {
	// Validate that the requested name for the CSI driver isn't already claimed by an existing CSI driver
	// (Can happen if a driver with an identical name was created in a different namespace)
	csiDriver := storagev1.CSIDriver{}
	csiDriver.Name = r.driver.Name
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(&csiDriver), &csiDriver); client.IgnoreNotFound(err) != nil {
		r.log.Error(err, "Failed to query the existence of a CSI Driver")
		return err
	}
	if csiDriver.UID != "" {
		// If the k8s csi driver the we found does not have an owner ref annotation
		// we assume someone else is owning it
		ownerRef := csiDriver.GetAnnotations()[ownerRefAnnotationKey]
		if ownerRef == "" {
			err := fmt.Errorf("invalid driver name")
			r.log.Error(err, "Desired name already in use by a different CSI Driver")
			return err
		}

		ownerObjKey := client.ObjectKey{}
		if err := json.Unmarshal([]byte(ownerRef), &ownerObjKey); err != nil {
			r.log.Error(err, "Failed to parse owner ref annotation on CSI Driver")
			return err
		}

		// If the owner annotation does not correlate with the driver we are reconciling
		// we assume there is another driver CR with an identical name on some other namespace
		if r.driver.Namespace != ownerObjKey.Namespace || r.driver.Name != ownerObjKey.Name {
			err := fmt.Errorf("invalid driver name")
			r.log.Error(err, "Desired name already in use by a different CSI Driver")
			return err
		}
	}

	// Extract the driver sort name and driver type
	matches := nameRegExp.FindStringSubmatch(r.driver.Name)
	if len(matches) != 3 {
		return fmt.Errorf("invalid driver name")
	}
	r.driverName = matches[1]
	r.driverType = strings.ToLower(matches[2])

	// Load operator configuration resource
	opConfig := csiv1a1.OperatorConfig{}
	opConfig.Name = operatorConfigName
	opConfig.Namespace = operatorNamespace
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(&opConfig), &opConfig); client.IgnoreNotFound(err) != nil {
		r.log.Error(err, "Unable to load operatorconfig.csi.ceph.io", "name", client.ObjectKeyFromObject(&opConfig))
		return err
	}

	// Load the current desired state in the form of a ceph csi driver resource
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(&r.driver), &r.driver); err != nil {
		r.log.Error(err, "Unable to load driver.csi.ceph.io object", "name", client.ObjectKeyFromObject(&r.driver))
		return err
	}

	// Creating a copy of the driver spec, making sure any local changes will not effect the object residing
	// in the client's cache
	r.driver.Spec = *r.driver.Spec.DeepCopy()
	if opConfig.Spec.DriverSpecDefaults != nil {
		mergeDriverSpecs(&r.driver.Spec, opConfig.Spec.DriverSpecDefaults)
	}
	mergeDriverSpecs(&r.driver.Spec, &driverDefaults)

	r.images = maps.Clone(imageDefaults)
	if r.driver.Spec.ImageSet != nil {
		imageSetCM := corev1.ConfigMap{}
		imageSetCM.Name = r.driver.Spec.ImageSet.Name
		imageSetCM.Namespace = r.driver.Namespace
		if err := r.Get(r.ctx, client.ObjectKeyFromObject(&imageSetCM), &imageSetCM); err != nil {
			r.log.Error(err, "Unable to load driver specified image set config map", "name", client.ObjectKeyFromObject(&imageSetCM))
			return err
		}

		maps.Copy(r.images, imageSetCM.Data)
	}

	return nil
}

func (r *driverReconcile) reconcileK8sCsiDriver() error {
	existingCsiDriver := &storagev1.CSIDriver{}
	existingCsiDriver.Name = r.driver.Name

	log := r.log.WithValues("driverName", existingCsiDriver.Name)
	log.Info("Reconciling CSI Driver")

	if err := r.Get(r.ctx, client.ObjectKeyFromObject(existingCsiDriver), existingCsiDriver); client.IgnoreNotFound(err) != nil {
		log.Error(err, "Failed to load CSI Driver resource")
		return err
	}

	desiredCsiDriver := &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name:        existingCsiDriver.Name,
			Labels:      existingCsiDriver.Labels,
			Annotations: existingCsiDriver.Annotations,
		},
	}

	// There is a need to start with a copy of the existing driver sepc becuase the
	// spec is being edited by another party. That in turn fails the equality check
	// unless we gureente that we started from the same point.
	existingCsiDriver.Spec.DeepCopyInto(&desiredCsiDriver.Spec)
	desiredCsiDriver.Spec.PodInfoOnMount = ptr.To(false)
	desiredCsiDriver.Spec.AttachRequired = utils.FirstNonNil(
		r.driver.Spec.AttachRequired,
		ptr.To(true),
	)
	desiredCsiDriver.Spec.FSGroupPolicy = ptr.To(
		utils.FirstNonEmpty(
			r.driver.Spec.FsGroupPolicy,
			r.driver.Spec.FsGroupPolicy,
			storagev1.FileFSGroupPolicy,
		),
	)
	if nodePlugin := r.driver.Spec.NodePlugin; nodePlugin != nil {
		desiredCsiDriver.Spec.SELinuxMount = nodePlugin.EnableSeLinuxHostMount
	}

	ownerObjKey := client.ObjectKeyFromObject(&r.driver)
	if bytes, err := json.Marshal(ownerObjKey); err == nil {
		if utils.AddAnnotation(desiredCsiDriver, ownerRefAnnotationKey, string(bytes)) {
			log.Info("ownerref annotation added to CSI driver resource")
		}
	} else {
		log.Error(
			err,
			"Failed to JSON marshal owner obj key for CSI driver resource",
			"ownerObjKey",
			ownerObjKey,
		)
		return err
	}

	if !reflect.DeepEqual(desiredCsiDriver.Spec, existingCsiDriver.Spec) {
		if existingCsiDriver.UID != "" {
			log.Info("CSI Driver resource exist but does not meet desired state")
			if err := r.Delete(r.ctx, existingCsiDriver); err != nil {
				log.Error(err, "Failed to delete existing CSI Driver resource")
				return err
			}
			log.Info("CSI Driver resource deleted successfully")
		} else {
			log.Info("CSI Driver resource does not exist")
		}

		if err := r.Create(r.ctx, desiredCsiDriver); err != nil {
			log.Error(err, "Failed to create a CSI Driver resource")
			return err
		}

		log.Info("CSI Driver resource created successfully")
	} else {
		log.Info("CSI Driver resource already meets desired state")
	}

	return nil
}

func (r *driverReconcile) reconcilePluginDeamonSet() error {
	return nil
}

func (r *driverReconcile) reconcileProvisionerDeployment() error {
	return nil
}

func (r *driverReconcile) reconcileLivnessService() error {
	return nil
}

// mergeDriverSpecs will fill in any unset fields in dest with a copy of the same field in src
func mergeDriverSpecs(dest, src *csiv1a1.DriverSpec) {
	// Create a copy of the src, making sure that any value copied into dest is a not shared
	// with the original src
	src = src.DeepCopy()

	if dest.Log == nil {
		dest.Log = src.Log
	}
	if dest.ImageSet == nil {
		dest.ImageSet = src.ImageSet
	}
	if dest.ClusterName == nil {
		dest.ClusterName = src.ClusterName
	}
	if dest.EnableMetadata == nil {
		dest.EnableMetadata = src.EnableMetadata
	}
	if dest.GRpcTimeout == 0 {
		dest.GRpcTimeout = src.GRpcTimeout
	}
	if dest.SnapshotPolicy == "" {
		dest.SnapshotPolicy = src.SnapshotPolicy
	}
	if dest.GenerateOMapInfo == nil {
		dest.GenerateOMapInfo = src.GenerateOMapInfo
	}
	if dest.FsGroupPolicy == "" {
		dest.FsGroupPolicy = src.FsGroupPolicy
	}
	if dest.Encryption == nil {
		dest.Encryption = src.Encryption
	}
	if src.NodePlugin != nil {
		if dest.NodePlugin == nil {
			dest.NodePlugin = src.NodePlugin
		} else {
			dest, src := dest.NodePlugin, src.NodePlugin
			if dest.PrioritylClassName == nil {
				dest.PrioritylClassName = src.PrioritylClassName
			}
			if dest.Labels == nil {
				dest.Labels = src.Labels
			}
			if dest.Annotations == nil {
				dest.Annotations = src.Annotations
			}
			if dest.Affinity == nil {
				dest.Affinity = src.Affinity
			}
			if dest.Tolerations == nil {
				dest.Tolerations = src.Tolerations
			}
			if dest.UpdateStrategy == nil {
				dest.UpdateStrategy = src.UpdateStrategy
			}
			if dest.Volumes == nil {
				dest.Volumes = src.Volumes
			}
			if dest.KubeletDirPath == "" {
				dest.KubeletDirPath = src.KubeletDirPath
			}
			if dest.EnableSeLinuxHostMount == nil {
				dest.EnableSeLinuxHostMount = src.EnableSeLinuxHostMount
			}
			if dest.ImagePullPolicy == "" {
				dest.ImagePullPolicy = src.ImagePullPolicy
			}
			if dest.Resources.Registrar == nil {
				dest.Resources.Registrar = src.Resources.Registrar
			}
			if dest.Resources.Liveness == nil {
				dest.Resources.Liveness = src.Resources.Liveness
			}
			if dest.Resources.Plugin == nil {
				dest.Resources.Plugin = src.Resources.Plugin
			}
		}
	}
	if src.ControllerPlugin != nil {
		if dest.ControllerPlugin == nil {
			dest.ControllerPlugin = src.ControllerPlugin
		} else {
			dest, src := dest.ControllerPlugin, src.ControllerPlugin
			if dest.PrioritylClassName == nil {
				dest.PrioritylClassName = src.PrioritylClassName
			}
			if dest.Labels == nil {
				dest.Labels = src.Labels
			}
			if dest.Annotations == nil {
				dest.Annotations = src.Annotations
			}
			if dest.Affinity == nil {
				dest.Affinity = src.Affinity
			}
			if dest.Tolerations == nil {
				dest.Tolerations = src.Tolerations
			}
			if dest.Replicas == nil {
				dest.Replicas = src.Replicas
			}
			if dest.Resources.Attacher == nil {
				dest.Resources.Attacher = src.Resources.Attacher
			}
			if dest.Resources.Snapshotter == nil {
				dest.Resources.Snapshotter = src.Resources.Snapshotter
			}
			if dest.Resources.Resizer == nil {
				dest.Resources.Resizer = src.Resources.Resizer
			}
			if dest.Resources.Provisioner == nil {
				dest.Resources.Provisioner = src.Resources.Provisioner
			}
			if dest.Resources.OMapGenerator == nil {
				dest.Resources.OMapGenerator = src.Resources.OMapGenerator
			}
			if dest.Resources.Liveness == nil {
				dest.Resources.Liveness = src.Resources.Liveness
			}
			if dest.Resources.Plugin == nil {
				dest.Resources.Plugin = src.Resources.Plugin
			}
		}
	}
	if dest.AttachRequired == nil {
		dest.AttachRequired = src.AttachRequired
	}
	if dest.Liveness == nil {
		dest.Liveness = src.Liveness
	}
	if dest.LeaderElection == nil {
		dest.LeaderElection = src.LeaderElection
	}
	if dest.DeployCsiAddons == nil {
		dest.DeployCsiAddons = src.DeployCsiAddons
	}
	if dest.KernelMountOptions == nil {
		dest.KernelMountOptions = src.KernelMountOptions
	}
	if src.CephFsClientType != "" {
		dest.CephFsClientType = src.CephFsClientType
	}
}
