/*
Copyright 2016 The Kubernetes Authors.

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

package simulator

import (
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/autoscaler/cluster-autoscaler/utils/daemonset"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"

	pod_util "k8s.io/autoscaler/cluster-autoscaler/utils/pod"
)

// NodeStartupPods returns a list of pods which are always expected to be scheduled on a given Node. The list is based on
// the provided pods currently scheduled on the pods. If forceDaemonSets is true, fake "missing" DS pods are added to the
// list for DaemonSets that don't have a pod running on the Node but should have.
//
// scheduledPods are not modified, the returned pods are not sanitized.
func NodeStartupPods(node *apiv1.Node, ndr schedulerframework.NodeDynamicResources, scheduledPods []*apiv1.Pod, daemonsets []*appsv1.DaemonSet, forceDaemonSets bool) ([]*apiv1.Pod, errors.AutoscalerError) {
	nodeInfo := schedulerframework.NewNodeInfo()
	nodeInfo.SetNode(node)
	nodeInfo.SetDynamicResources(ndr)
	return getStartupPods(nodeInfo, scheduledPods, daemonsets, forceDaemonSets)
}

func getStartupPods(nodeInfo *schedulerframework.NodeInfo, scheduledPods []*apiv1.Pod, daemonsets []*appsv1.DaemonSet, forceDaemonSets bool) ([]*apiv1.Pod, errors.AutoscalerError) {
	var result []*apiv1.Pod
	runningDS := make(map[types.UID]bool)
	for _, pod := range scheduledPods {
		// Ignore scheduled pods in deletion phase
		if pod.DeletionTimestamp != nil {
			continue
		}
		// Add scheduled mirror and DS pods
		if pod_util.IsMirrorPod(pod) || pod_util.IsDaemonSetPod(pod) {
			nodeInfo.AddPod(pod)
			result = append(result, pod)
		}
		// Mark DS pods as running
		controllerRef := metav1.GetControllerOf(pod)
		if controllerRef != nil && controllerRef.Kind == "DaemonSet" {
			runningDS[controllerRef.UID] = true
		}
	}
	// Add all pending DS pods if force scheduling DS
	if forceDaemonSets {
		var pendingDS []*appsv1.DaemonSet
		for _, ds := range daemonsets {
			if !runningDS[ds.UID] {
				pendingDS = append(pendingDS, ds)
			}
		}
		daemonPods, err := daemonset.GetDaemonSetPodsForNode(nodeInfo, pendingDS)
		if err != nil {
			return nil, errors.ToAutoscalerError(errors.InternalError, err)
		}
		for _, pod := range daemonPods {
			result = append(result, pod)
		}
	}
	return result, nil
}
