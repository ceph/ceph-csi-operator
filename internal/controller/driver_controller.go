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
	"slices"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	csiv1a1 "github.com/ceph/ceph-csi-operator/api/v1alpha1"
	"github.com/ceph/ceph-csi-operator/internal/utils"
)

//+kubebuilder:rbac:groups=csi.ceph.io,resources=drivers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csi.ceph.io,resources=drivers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csi.ceph.io,resources=drivers/finalizers,verbs=update
//+kubebuilder:rbac:groups=csi.ceph.io,resources=operatorconfigs,verbs=get;list;watch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

type DriverType string

const (
	RbdDriverType    = "rbd"
	CephFsDriverType = "cephfs"
	NfsDriverType    = "nfs"
)

const ownerRefAnnotationKey = "csi.ceph.io/ownerref"

// A regexp used to parse driver short name and driver type from the
// driver's full name
var nameRegExp, _ = regexp.Compile(fmt.Sprintf(
	`^(?:(.+)\.)?(%s|%s|%s)\.csi\.ceph\.com$`,
	RbdDriverType,
	CephFsDriverType,
	NfsDriverType,
))

// DriverReconciler reconciles a Driver object
type DriverReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// A local reconcile object tied to a single reconcile iteration
type driverReconcile struct {
	DriverReconciler

	ctx          context.Context
	log          logr.Logger
	driver       csiv1a1.Driver
	driverPrefix string
	driverType   DriverType
	images       map[string]string
}

// SetupWithManager sets up the controller with the Manager.
func (r *DriverReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// Define conditions for an OperatorConfig change that the require queuing of reconciliation
	// request for drivers
	driverDefaultsPredicate := builder.WithPredicates(
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				opConf, ok := e.Object.(*csiv1a1.OperatorConfig)
				return ok && opConf.Spec.DriverSpecDefaults != nil
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				old, oldOk := e.ObjectOld.(*csiv1a1.OperatorConfig)
				new, newOk := e.ObjectNew.(*csiv1a1.OperatorConfig)
				return oldOk && newOk &&
					!reflect.DeepEqual(old.Spec.DriverSpecDefaults, new.Spec.DriverSpecDefaults)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				opConf, ok := e.Object.(*csiv1a1.OperatorConfig)
				return ok && opConf.Spec.DriverSpecDefaults != nil
			},
			GenericFunc: func(event.GenericEvent) bool {
				return false
			},
		},
	)

	// Enqueue a reconcile request for all existing drivers, used to trigger a reconcile
	// for all drivers  whenever the driver default configuration changes
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
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{}).
		Watches(&csiv1a1.OperatorConfig{}, enqueueAllDrivers, driverDefaultsPredicate).
		Watches(&storagev1.CSIDriver{}, enqueueFromOwnerRefAnnotation).
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
		r.reconcileControllerPluginDeployment,
		r.reconcileNodePluginDeamonSet,
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
		ownerObjKey := client.ObjectKey{}
		if ownerRef := csiDriver.GetAnnotations()[ownerRefAnnotationKey]; ownerRef != "" {
			if err := json.Unmarshal([]byte(ownerRef), &ownerObjKey); err != nil {
				r.log.Error(err, "Failed to parse owner ref annotation on CSI Driver")
				return err
			}
		}
		if csiDriver.Namespace != r.driver.Namespace || csiDriver.Name != r.driver.Name {
			err := fmt.Errorf("invalid driver name")
			r.log.Error(err, "Desired name already in use by a different CSI Driver")
		}
	}

	// Load operator configuration resource
	opConfig := csiv1a1.OperatorConfig{}
	opConfig.Name = operatorConfigName
	opConfig.Namespace = operatorNamespace
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(&opConfig), &opConfig); client.IgnoreNotFound(err) != nil {
		r.log.Error(err, "Unable to load operatorconfig.csi.ceph.io", "name", client.ObjectKeyFromObject(&opConfig))
		return err
	}

	// Extract the driver sort name and driver type
	matches := nameRegExp.FindStringSubmatch(r.driver.Name)
	if len(matches) != 3 {
		return fmt.Errorf("invalid driver name")
	}
	r.driverPrefix = matches[1]
	r.driverType = DriverType(strings.ToLower(matches[2]))

	// Load the current desired state in the form of a ceph csi driver resource
	if err := r.Get(r.ctx, client.ObjectKeyFromObject(&r.driver), &r.driver); err != nil {
		r.log.Error(err, "Unable to load driver.csi.ceph.io object", "name", client.ObjectKeyFromObject(&r.driver))
		return client.IgnoreNotFound(err)
	}

	// Creating a copy of the driver spec, making sure any local changes will not effect the object residing
	// in the client's cache
	r.driver.Spec = *r.driver.Spec.DeepCopy()
	mergeDriverSpecs(&r.driver.Spec, opConfig.Spec.DriverSpecDefaults)

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

	desiredCsiDriver := existingCsiDriver.DeepCopy()
	desiredCsiDriver.Spec = storagev1.CSIDriverSpec{
		PodInfoOnMount: ptr.To(false),
		AttachRequired: utils.If(
			r.driver.Spec.AttachRequired != nil,
			r.driver.Spec.AttachRequired,
			ptr.To(true),
		),
		FSGroupPolicy: utils.If(
			r.driver.Spec.FsGroupPolicy != "",
			ptr.To(r.driver.Spec.FsGroupPolicy),
			ptr.To(storagev1.FileFSGroupPolicy),
		),
	}

	ownerObjKey := client.ObjectKeyFromObject(&r.driver)
	if bytes, err := json.Marshal(ownerObjKey); err != nil {
		log.Error(
			err,
			"Failed to JSON marshal owner obj key for CSI driver resource",
			"ownerObjKey",
			ownerObjKey,
		)
		return err
	} else {
		utils.AddAnnotation(desiredCsiDriver, ownerRefAnnotationKey, string(bytes))
	}

	if existingCsiDriver.UID == "" || !reflect.DeepEqual(desiredCsiDriver, existingCsiDriver) {
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

func (r *driverReconcile) reconcileControllerPluginDeployment() error {
	deploy := &appsv1.Deployment{}
	deploy.Name = r.generateName("ctrlplugin")
	deploy.Namespace = r.driver.Namespace

	log := r.log.WithValues("deploymentName", deploy.Name)
	log.Info("Reconciling controller plugin deployment")

	opResult, err := ctrlutil.CreateOrUpdate(r.ctx, r.Client, deploy, func() error {
		r.log.Info("Controller plugin deployment reconciled successfully")
		if err := ctrlutil.SetOwnerReference(&r.driver, deploy, r.Scheme); err != nil {
			log.Error(err, "Failed setting an owner reference on deployment")
			return err
		}

		appName := deploy.Name
		leaderElectionSpec := utils.FirstNonNil(r.driver.Spec.LeaderElection, &defaultLeaderElection)
		pluginSpec := utils.FirstNonNil(r.driver.Spec.ControllerPlugin, &csiv1a1.ControllerPluginSpec{})
		serviceAccountName := utils.FirstNonEmpty(
			ptr.Deref(pluginSpec.ServiceAccountName, ""),
			fmt.Sprintf("csi-%s-ctrlplugin-sa", r.driverType),
		)
		imagePullPolicy := utils.FirstNonEmpty(pluginSpec.ImagePullPolicy, corev1.PullIfNotPresent)
		grpcTimeout := utils.FirstNonZero(r.driver.Spec.GRpcTimeout, defaultGRrpcTimeout)
		logLevel := utils.If(r.driver.Spec.Log != nil, r.driver.Spec.Log.LogLevel, 0)
		forceKernelClient := r.isCephFsDriver() && r.driver.Spec.CephFsClientType == csiv1a1.KernelCephFsClient

		leaderElectionArgs := []string{
			utils.LeaderElectionContainerArg,
			utils.LeaderElectionNamespaceContainerArg(r.driver.Namespace),
			utils.LeaderElectionLeaseDurationContainerArg(leaderElectionSpec.LeaseDuration),
			utils.LeaderElectionRenewDeadlineContainerArg(leaderElectionSpec.RenewDeadline),
			utils.LeaderElectionRetryPeriodContainerArg(leaderElectionSpec.RetryPeriod),
		}

		deploy.Spec = appsv1.DeploymentSpec{
			Replicas: pluginSpec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": appName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.Call(func() map[string]string {
						podLabels := map[string]string{}
						maps.Copy(podLabels, pluginSpec.Labels)
						podLabels["app"] = appName
						return podLabels
					}),
					Annotations: maps.Clone(pluginSpec.Annotations),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					PriorityClassName:  ptr.Deref(pluginSpec.PrioritylClassName, ""),
					Affinity:           pluginSpec.Affinity,
					Tolerations:        pluginSpec.Tolerations,
					Containers: utils.Call(func() []corev1.Container {
						containers := []corev1.Container{
							// Plugin Container
							{
								Name:            fmt.Sprintf("csi-%splugin", r.driverType),
								Image:           r.images["plugin"],
								ImagePullPolicy: imagePullPolicy,
								Args: []string{
									utils.TypeContainerArg(string(r.driverType)),
									utils.LogLevelContainerArg(logLevel),
									utils.EndpointContainerArg,
									utils.NodeIdContainerArg,
									utils.ControllerServerContainerArg,
									utils.DriverNameContainerArg(r.driver.Name),
									utils.PidlimitContainerArg,
									utils.SetMetadataContainerArg(ptr.Deref(r.driver.Spec.EnableMetadata, false)),
									utils.ClusterNameContainerArg(ptr.Deref(r.driver.Spec.ClusterName, "")),
									utils.If(forceKernelClient, utils.ForceCephKernelClientContainerArg, ""),
									utils.If(
										ptr.Deref(r.driver.Spec.DeployCsiAddons, false),
										utils.CsiAddonsEndpointContainerArg,
										"",
									),
								},
								Env: []corev1.EnvVar{
									utils.PodIpEnvVar,
									utils.NodeIdEnvVar,
									utils.PodNamespaceEnvVar,
								},
								VolumeMounts: utils.Call(func() []corev1.VolumeMount {
									mounts := append(
										// Add user defined volume mounts at the start to make sure they do not
										// overwrite built in volumes mounts.
										utils.MapSlice(
											pluginSpec.Volumes,
											func(v csiv1a1.VolumeSpec) corev1.VolumeMount {
												return v.Mount
											},
										),
										utils.SocketDirVolumeMount,
										utils.HostDevVolumeMount,
										utils.HostSysVolumeMount,
										utils.LibModulesVolumeMount,
										utils.KeysTmpDirVolumeMount,
										utils.CsiConfigVolumeMount,
									)
									if r.driver.Spec.Encryption != nil {
										mounts = append(mounts, utils.KmsConfigsVolumeMount)
									}
									if r.isRdbDriver() {
										mounts = append(mounts, utils.OidcTokenVolumeMount)
									}
									return mounts
								}),
								Resources: ptr.Deref(
									pluginSpec.Resources.Plugin,
									corev1.ResourceRequirements{},
								),
							},
							// Provisioner Sidecar Container
							{
								Name:            "csi-provisioner",
								ImagePullPolicy: imagePullPolicy,
								Image:           r.images["provisioner"],
								Args: append(
									slices.Clone(leaderElectionArgs),
									utils.LogLevelContainerArg(logLevel),
									utils.CsiAddressContainerArg,
									utils.TimeoutContainerArg(grpcTimeout),
									utils.RetryIntervalStartContainerArg,
									utils.DefaultFsTypeContainerArg,
									utils.PreventVolumeModeConversionContainerArg,
									utils.HonorPVReclaimPolicyContainerArg,
									utils.If(r.isRdbDriver(), utils.DefaultFsTypeContainerArg, ""),
									utils.If(r.isRdbDriver(), utils.TopologyContainerArg, ""),
									utils.If(!r.isNfsDriver(), utils.ExtraCreateMetadataContainerArg, ""),
								),
								VolumeMounts: []corev1.VolumeMount{
									utils.SocketDirVolumeMount,
								},
								Resources: ptr.Deref(
									pluginSpec.Resources.Provisioner,
									corev1.ResourceRequirements{},
								),
							},
							// Resizer Sidecar Container
							{
								Name:            "csi-resizer",
								ImagePullPolicy: imagePullPolicy,
								Image:           r.images["resizer"],
								Args: append(
									slices.Clone(leaderElectionArgs),
									utils.LogLevelContainerArg(logLevel),
									utils.CsiAddressContainerArg,
									utils.TimeoutContainerArg(r.driver.Spec.GRpcTimeout),
									utils.HandleVolumeInuseErrorContainerArg,
									utils.RecoverVolumeExpansionFailureContainerArg,
								),
								VolumeMounts: []corev1.VolumeMount{
									utils.SocketDirVolumeMount,
								},
								Resources: ptr.Deref(
									pluginSpec.Resources.Resizer,
									corev1.ResourceRequirements{},
								),
							},
							// Attacher Sidecar Container
							{
								Name:            "csi-attacher",
								ImagePullPolicy: imagePullPolicy,
								Image:           r.images["attacher"],
								Args: append(
									slices.Clone(leaderElectionArgs),
									utils.LogLevelContainerArg(logLevel),
									utils.CsiAddressContainerArg,
									utils.TimeoutContainerArg(grpcTimeout),
									utils.If(r.isRdbDriver(), utils.DefaultFsTypeContainerArg, ""),
								),
								VolumeMounts: []corev1.VolumeMount{
									utils.SocketDirVolumeMount,
								},
								Resources: ptr.Deref(
									pluginSpec.Resources.Attacher,
									corev1.ResourceRequirements{},
								),
							},
							// Snapshotter Sidecar Container
							{
								Name:            "csi-snapshotter",
								ImagePullPolicy: imagePullPolicy,
								Image:           r.images["snapshotter"],
								Args: append(
									slices.Clone(leaderElectionArgs),
									utils.LogLevelContainerArg(logLevel),
									utils.CsiAddressContainerArg,
									utils.TimeoutContainerArg(grpcTimeout),
									utils.If(r.isNfsDriver(), utils.ExtraCreateMetadataContainerArg, ""),
									utils.If(
										r.driverType != NfsDriverType,
										utils.EnableVolumeGroupSnapshotsContainerArg,
										"",
									),
								),
								VolumeMounts: []corev1.VolumeMount{
									utils.SocketDirVolumeMount,
								},
								Resources: ptr.Deref(
									pluginSpec.Resources.Snapshotter,
									corev1.ResourceRequirements{},
								),
							},
						}
						// Addons Sidecar Container
						if ptr.Deref(r.driver.Spec.DeployCsiAddons, false) {
							containers = append(containers, corev1.Container{
								Name:            "csi-addons",
								Image:           r.images["addons"],
								ImagePullPolicy: imagePullPolicy,
								Args: append(
									slices.Clone(leaderElectionArgs),
									utils.LogLevelContainerArg(logLevel),
									utils.NodeIdContainerArg,
									utils.PodContainerArg,
									utils.PodUidContainerArg,
									utils.CsiAddonsAddressContainerArg,
									utils.ControllerPortContainerArg,
									utils.NamespaceContainerArg,
									fmt.Sprintf("--log_file=%s/log/%s/csi-addons.log", "/var/lib/cephcsi", deploy.Name),
								),
								Ports: []corev1.ContainerPort{
									utils.CsiAddonsContainerPort,
								},
								Env: []corev1.EnvVar{
									utils.NodeIdEnvVar,
									utils.PodUidEnvVar,
									utils.PodNameEnvVar,
									utils.PodNamespaceEnvVar,
								},
								VolumeMounts: []corev1.VolumeMount{
									utils.SocketDirVolumeMount,
								},
								Resources: ptr.Deref(
									pluginSpec.Resources.Addons,
									corev1.ResourceRequirements{},
								),
							})
						}
						// OMap Generator Sidecar Container
						if r.isRdbDriver() && ptr.Deref(r.driver.Spec.GenerateOMapInfo, false) {
							containers = append(containers, corev1.Container{
								Name:            "csi-omap-generator",
								Image:           r.images["plugin"],
								ImagePullPolicy: imagePullPolicy,
								Args: []string{
									utils.LogLevelContainerArg(logLevel),
									utils.TypeContainerArg("controller"),
									utils.DriverNamespaceContainerArg,
									utils.DriverNameContainerArg(r.driver.Name),
									utils.SetMetadataContainerArg(ptr.Deref(r.driver.Spec.EnableMetadata, false)),
									utils.ClusterNameContainerArg(ptr.Deref(r.driver.Spec.ClusterName, "")),
								},
								Env: []corev1.EnvVar{
									utils.DriverNamespaceEnvVar,
								},
								VolumeMounts: []corev1.VolumeMount{
									utils.CsiConfigVolumeMount,
									utils.KeysTmpDirVolumeMount,
								},
								Resources: ptr.Deref(
									pluginSpec.Resources.OMapGenerator,
									corev1.ResourceRequirements{},
								),
							})
						}
						// Liveness Sidecar Container
						if r.driver.Spec.Liveness != nil {
							containers = append(containers, corev1.Container{
								Name:            "liveness-prometheus",
								Image:           r.images["plugin"],
								ImagePullPolicy: imagePullPolicy,
								Args: []string{
									utils.TypeContainerArg("liveness"),
									utils.EndpointContainerArg,
									utils.MetricsPortContainerArg(r.driver.Spec.Liveness.MetricsPort),
									utils.MetricsPathContainerArg,
									utils.PoolTimeContainerArg,
									utils.TimeoutContainerArg(3),
								},
								Env: []corev1.EnvVar{
									utils.PodIpEnvVar,
								},
								VolumeMounts: []corev1.VolumeMount{
									utils.SocketDirVolumeMount,
								},
								Resources: ptr.Deref(
									pluginSpec.Resources.Liveness,
									corev1.ResourceRequirements{},
								),
							})
						}

						return containers
					}),
					Volumes: utils.Call(func() []corev1.Volume {
						volumes := append(
							// Add user defined volumes at the start to make sure they do not
							// overwrite built in volumes.
							utils.MapSlice(
								pluginSpec.Volumes,
								func(v csiv1a1.VolumeSpec) corev1.Volume {
									return v.Volume
								},
							),
							utils.HostDevVolume,
							utils.HostSysVolume,
							utils.LibModulesVolume,
							utils.SocketDirVolume,
							utils.KeysTmpDirVolume,
							utils.OidcTokenVolume,
							utils.CsiConfigsVolume(&corev1.LocalObjectReference{
								Name: fmt.Sprintf("%s-csi-configs", r.driver.Name),
							}),
						)
						if r.driver.Spec.Encryption != nil {
							volumes = append(
								volumes,
								utils.KmsConfigVolume(&r.driver.Spec.Encryption.ConfigMapRef))
						}
						return volumes
					}),
				},
			},
		}

		return nil
	})

	r.logCreateOrUpdateResult(r.log, deploy, opResult, err)
	return nil
}

func (r *driverReconcile) reconcileNodePluginDeamonSet() error {
	daemonSet := &appsv1.DaemonSet{}
	daemonSet.Name = r.generateName("nodeplugin")
	daemonSet.Namespace = r.driver.Namespace

	log := r.log.WithValues("daemonSetName", daemonSet.Name)
	log.Info("Reconciling controller plugin deployment")

	opResult, err := ctrlutil.CreateOrUpdate(r.ctx, r.Client, daemonSet, func() error {
		if err := ctrlutil.SetOwnerReference(&r.driver, daemonSet, r.Scheme); err != nil {
			log.Error(err, "Failed setting an owner reference on deployment")
			return err
		}

		appName := daemonSet.Name
		pluginSpec := utils.FirstNonNil(r.driver.Spec.NodePlugin, &csiv1a1.NodePluginSpec{})
		serviceAccountName := utils.FirstNonEmpty(
			ptr.Deref(pluginSpec.ServiceAccountName, ""),
			fmt.Sprintf("csi-%s-nodeplugin-sa", r.driverType),
		)
		imagePullPolicy := utils.FirstNonEmpty(pluginSpec.ImagePullPolicy, corev1.PullIfNotPresent)
		logLevel := utils.If(r.driver.Spec.Log != nil, r.driver.Spec.Log.LogLevel, 0)
		kubeletDirPath := utils.FirstNonEmpty(pluginSpec.KubeletDirPath, defaultKubeletDirPath)
		forceKernelClient := r.isCephFsDriver() && r.driver.Spec.CephFsClientType == csiv1a1.KernelCephFsClient

		daemonSet.Spec = appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": appName},
			},
			UpdateStrategy: ptr.Deref(pluginSpec.UpdateStrategy, defautUpdateStrategy),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.Call(func() map[string]string {
						podLabels := map[string]string{}
						maps.Copy(podLabels, pluginSpec.Labels)
						podLabels["app"] = appName
						if r.driver.Spec.Liveness != nil {
							podLabels["contains"] = fmt.Sprintf("%s-metrics", appName)
						}
						return podLabels
					}),
					Annotations: maps.Clone(pluginSpec.Annotations),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					PriorityClassName:  ptr.Deref(pluginSpec.PrioritylClassName, ""),
					HostNetwork:        true,
					HostPID:            r.isRdbDriver(),
					// to use e.g. Rook orchestrated cluster, and mons' FQDN is
					// resolved through k8s service, set dns policy to cluster first
					DNSPolicy: corev1.DNSClusterFirstWithHostNet,
					Containers: utils.Call(func() []corev1.Container {
						containers := []corev1.Container{
							// Node Plugin Container
							{
								Name:            fmt.Sprintf("csi-%splugin", r.driverType),
								Image:           r.images["plugin"],
								ImagePullPolicy: imagePullPolicy,
								SecurityContext: &corev1.SecurityContext{
									Privileged: ptr.To(true),
									Capabilities: &corev1.Capabilities{
										Add:  []corev1.Capability{"SYS_ADMIN"},
										Drop: []corev1.Capability{"All"},
									},
									AllowPrivilegeEscalation: ptr.To(true),
								},
								Args: []string{
									utils.LogLevelContainerArg(logLevel),
									utils.TypeContainerArg(string(r.driverType)),
									utils.NodeServerContainerArg,
									utils.NodeIdContainerArg,
									utils.DriverNameContainerArg(r.driver.Name),
									utils.EndpointContainerArg,
									utils.PidlimitContainerArg,
									utils.If(forceKernelClient, utils.ForceCephKernelClientContainerArg, ""),
									utils.If(ptr.Deref(r.driver.Spec.DeployCsiAddons, false), utils.CsiAddonsEndpointContainerArg, ""),
									utils.If(r.isRdbDriver(), utils.StagingPathContainerArg(kubeletDirPath), ""),
									utils.If(r.isCephFsDriver(), utils.KernelMountOptionsContainerArg(r.driver.Spec.KernelMountOptions), ""),
									utils.If(r.isCephFsDriver(), utils.FuseMountOptionsContainerArg(r.driver.Spec.FuseMountOptions), ""),
									// TODO: RBD only, add "--domainlabels={{ .CSIDomainLabels }}". not sure hot to get the info
								},
								Env: []corev1.EnvVar{
									utils.PodIpEnvVar,
									utils.NodeIdEnvVar,
									utils.PodNamespaceEnvVar,
								},
								VolumeMounts: utils.Call(func() []corev1.VolumeMount {
									mounts := []corev1.VolumeMount{
										utils.HostDevVolumeMount,
										utils.HostSysVolumeMount,
										utils.HostRunMountVolumeMount,
										utils.LibModulesVolumeMount,
										utils.KeysTmpDirVolumeMount,
										utils.PluginDirVolumeMount,
										utils.PluginMountDirVolumeMount(kubeletDirPath),
										utils.PodsMountDirVolumeMount(kubeletDirPath),
									}
									if ptr.Deref(pluginSpec.EnableSeLinuxHostMount, false) {
										mounts = append(mounts, utils.EtcSelinuxVolumeMount)
									}
									if r.driver.Spec.Encryption != nil {
										mounts = append(mounts, utils.KmsConfigsVolumeMount)
									}
									if r.isRdbDriver() {
										mounts = append(mounts, utils.OidcTokenVolumeMount)
									}
									return mounts
								}),
								Resources: ptr.Deref(
									pluginSpec.Resources.Plugin,
									corev1.ResourceRequirements{},
								),
							},
							// Registrar Sidecar Container
							{
								Name:            "driver-registrar",
								Image:           r.images["registrar"],
								ImagePullPolicy: imagePullPolicy,
								// This is necessary only for systems with SELinux, where
								// non-privileged sidecar containers cannot access unix domain socket
								// created by privileged CSI driver container.
								SecurityContext: &corev1.SecurityContext{
									Privileged: ptr.To(true),
									Capabilities: &corev1.Capabilities{
										Drop: []corev1.Capability{"All"},
									},
								},
								Args: []string{
									utils.LogLevelContainerArg(logLevel),
									utils.KubeletRegistrationPathContainerArg(kubeletDirPath, r.driver.Name),
									utils.CsiAddressContainerArg,
								},
								VolumeMounts: []corev1.VolumeMount{
									utils.PluginDirVolumeMount,
									utils.RegistrationDirVolumeMount,
								},
								Resources: ptr.Deref(
									pluginSpec.Resources.Registrar,
									corev1.ResourceRequirements{},
								),
							},
						}
						// CSI Addons Sidecar Container
						if ptr.Deref(r.driver.Spec.DeployCsiAddons, false) {
							containers = append(containers, corev1.Container{
								Name:            "csi-addons",
								Image:           r.images["addons"],
								ImagePullPolicy: imagePullPolicy,
								SecurityContext: &corev1.SecurityContext{
									Privileged: ptr.To(true),
									Capabilities: &corev1.Capabilities{
										Drop: []corev1.Capability{"All"},
									},
								},
								Args: []string{
									utils.NodeIdContainerArg,
									utils.LogLevelContainerArg(logLevel),
									utils.CsiAddonsAddressContainerArg,
									utils.ControllerPortContainerArg,
									utils.PodContainerArg,
									utils.NamespaceContainerArg,
									utils.PodUidContainerArg,
									utils.StagingPathContainerArg(kubeletDirPath),
								},
								Ports: []corev1.ContainerPort{
									utils.CsiAddonsContainerPort,
								},
								Env: []corev1.EnvVar{
									utils.NodeIdEnvVar,
									utils.PodNameEnvVar,
									utils.PodNamespaceEnvVar,
									utils.PodUidEnvVar,
								},
								VolumeMounts: []corev1.VolumeMount{
									utils.PluginDirVolumeMount,
								},
								Resources: ptr.Deref(
									pluginSpec.Resources.Addons,
									corev1.ResourceRequirements{},
								),
							})
							// Liveness Sidecar Container
							if r.driver.Spec.Liveness != nil {
								containers = append(containers, corev1.Container{
									Name:            "liveness-prometheus",
									Image:           r.images["plugin"],
									ImagePullPolicy: imagePullPolicy,
									SecurityContext: &corev1.SecurityContext{
										Privileged: ptr.To(true),
										Capabilities: &corev1.Capabilities{
											Drop: []corev1.Capability{"All"},
										},
									},
									Args: []string{
										utils.TypeContainerArg("liveness"),
										utils.EndpointContainerArg,
										utils.MetricsPortContainerArg(r.driver.Spec.Liveness.MetricsPort),
										utils.MetricsPathContainerArg,
										utils.PoolTimeContainerArg,
										utils.TimeoutContainerArg(3),
									},
									Env: []corev1.EnvVar{
										utils.PodIpEnvVar,
									},
									VolumeMounts: []corev1.VolumeMount{
										utils.PluginDirVolumeMount,
									},
									Resources: ptr.Deref(
										pluginSpec.Resources.Liveness,
										corev1.ResourceRequirements{},
									),
								})
							}
						}
						return containers
					}),
				},
			},
		}

		return nil
	})

	r.logCreateOrUpdateResult(r.log, daemonSet, opResult, err)
	return nil
}

func (r *driverReconcile) reconcileLivnessService() error {
	if r.driver.Spec.Liveness == nil {
		return nil
	}

	service := &corev1.Service{}
	service.Namespace = r.driver.Namespace
	service.Name = r.generateName("livness")

	log := r.log.WithValues("service", service.Name)
	log.Info("Reconciling livness service")

	opResult, err := ctrlutil.CreateOrUpdate(r.ctx, r.Client, service, func() error {
		if err := ctrlutil.SetOwnerReference(&r.driver, service, r.Scheme); err != nil {
			r.log.Error(err, "Faild setting an owner reference on service")
			return err
		}

		service.Spec = corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "csi-http-metrics",
					Port:       8080,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(r.driver.Spec.Liveness.MetricsPort),
				},
			},
		}

		return nil
	})

	r.logCreateOrUpdateResult(r.log, service, opResult, err)
	return nil
}

func (r *driverReconcile) logCreateOrUpdateResult(
	log logr.Logger,
	obj client.Object,
	opRes ctrlutil.OperationResult,
	err error,
) {
	if err == nil {
		verb := utils.If(obj.GetUID() == "", "create", "update")
		r.log.Error(err, "Failed to %s resource", verb)
		return
	}

	switch opRes {
	case ctrlutil.OperationResultNone:
		r.log.Info("Resource is already up to date")
	case ctrlutil.OperationResultUpdated:
		r.log.Info("Resource successfully updated")
	case ctrlutil.OperationResultCreated:
		r.log.Info("Resource successfully created")
	}
}

func (r *driverReconcile) isRdbDriver() bool {
	return r.driverType == RbdDriverType
}

func (r *driverReconcile) isCephFsDriver() bool {
	return r.driverType == CephFsDriverType
}

func (r *driverReconcile) isNfsDriver() bool {
	return r.driverType == NfsDriverType
}

func (r *driverReconcile) generateName(suffix string) string {
	return fmt.Sprintf("%s-%s", r.driver.Name, suffix)
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
			if dest.ServiceAccountName == nil {
				dest.ServiceAccountName = src.ServiceAccountName
			}
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
			if dest.Volumes == nil {
				dest.Volumes = src.Volumes
			}
			if dest.ImagePullPolicy == "" {
				dest.ImagePullPolicy = src.ImagePullPolicy
			}
			if dest.UpdateStrategy == nil {
				dest.UpdateStrategy = src.UpdateStrategy
			}
			if dest.KubeletDirPath == "" {
				dest.KubeletDirPath = src.KubeletDirPath
			}
			if dest.EnableSeLinuxHostMount == nil {
				dest.EnableSeLinuxHostMount = src.EnableSeLinuxHostMount
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
			if dest.ServiceAccountName == nil {
				dest.ServiceAccountName = src.ServiceAccountName
			}
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
			if dest.Volumes == nil {
				dest.Volumes = src.Volumes
			}
			if dest.ImagePullPolicy == "" {
				dest.ImagePullPolicy = src.ImagePullPolicy
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
