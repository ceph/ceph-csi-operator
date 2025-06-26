package utils

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log logr.Logger

const (
	CSIAddonsDeploymentLabel     = "csi-addons-driver-type"
	CSICtrlpluginDeploymentLabel = "csi-ctrlplugin-driver-type"
)

// AddNodeNameIndexerForPods adds a field indexer on manager for pods identified
// by their nodeName
func AddNodeNameIndexerForPods(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod, ok := rawObj.(*corev1.Pod)
		if !ok {
			return nil
		}

		if pod.Spec.NodeName == "" {
			return nil
		}
		return []string{pod.Spec.NodeName}
	}); err != nil {
		log.Error(err, "failed to set field indexer for pods on manager")
		return err
	}

	log.Info("Successfully set node name indexer for pods on manager")
	return nil
}

// AddEventHandlerForPodDeletion watches for pod delete events.
// If the deleted pod is of csi ctrlplugin deployment the matching
// csi-addons deployment pod is deleted so that the scheduler could reevaluate
// pod placement
func AddEventHandlerForPodDeletion(c client.Client, mgr ctrl.Manager) error {
	podInformer, err := mgr.GetCache().GetInformer(context.Background(), &corev1.Pod{})
	if err != nil {
		log.Error(err, "failed to fetch informer for pods from manager")
		return err
	}

	if _, err = podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj any) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}

			driverType, found := pod.GetLabels()[CSICtrlpluginDeploymentLabel]
			if !found {
				return
			}

			log.Info("Encountered a delete event for csi ctrlplugin pod", "PodName", pod.GetName(), "NodeName", pod.Spec.NodeName, "DriverType", driverType)
			csiAddonsPodSelector := labels.SelectorFromSet(labels.Set{
				CSIAddonsDeploymentLabel: driverType,
			})

			listOpts := []client.ListOption{
				client.InNamespace(pod.GetNamespace()),
				client.MatchingLabelsSelector{Selector: csiAddonsPodSelector},
				client.MatchingFields{"spec.nodeName": pod.Spec.NodeName},
			}

			ctx := context.Background()
			podList := &corev1.PodList{}
			if err := c.List(ctx, podList, listOpts...); err != nil {
				log.Error(err, "failed to list pods with requested list options", "ListOptions", listOpts)
				return
			}

			podsLen := len(podList.Items)
			if podsLen == 0 || podsLen > 1 {
				log.Info("found unexpected number of pods with requested list options", "ListOptions", listOpts, "NumberOfPods", podsLen)
				return
			}

			for _, podToDelete := range podList.Items {
				if err := c.Delete(ctx, &podToDelete); client.IgnoreNotFound(err) != nil {
					log.Error(err, "failed to delete orphan csi-addons pod", "PodName", podToDelete.GetName())
					return
				}

				log.Info("Successfully removed orphaned csi-addons pod", "PodName", podToDelete.GetName(), "NodeName", podToDelete.Spec.NodeName)
			}

		},
	}); err != nil {
		log.Error(err, "failed to add event handler on pod informer")
		return err
	}

	log.Info("Successfully registered event handler for pod deletion events")
	return nil
}

// The functions defined in this file will be used before a logger
// is set on the reconciler, typically inside SetupWithManager.
// Set a logger identified by "ctrl-utils" on init
func init() {
	log = ctrl.Log.WithName("ctrl-utils")
}
