/*
Copyright 2024 The Kubernetes Authors.

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

package podinjection

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/clustersnapshot"
)

type podGroup struct {
	podCount        int
	desiredReplicas int
	sample          *clustersnapshot.PodResourceInfo
	ownerUid        types.UID
}

// groupPods creates a map of controller uids and podGroups.
// If a controller for some pods is not found, such pods are ignored and not grouped
func groupPods(pods []*clustersnapshot.PodResourceInfo, controllers []controller) map[types.UID]podGroup {
	podGroups := map[types.UID]podGroup{}
	for _, con := range controllers {
		podGroups[con.uid] = makePodGroup(con.desiredReplicas)
	}

	for _, pod := range pods {
		for _, ownerRef := range pod.Pod.OwnerReferences {
			podGroups = updatePodGroups(pod, ownerRef, podGroups)
		}
	}
	return podGroups
}

// updatePodGroups updates the pod group if ownerRef is the controller of the pod
func updatePodGroups(pod *clustersnapshot.PodResourceInfo, ownerRef metav1.OwnerReference, podGroups map[types.UID]podGroup) map[types.UID]podGroup {
	if ownerRef.Controller == nil {
		return podGroups
	}
	if !*(ownerRef.Controller) {
		return podGroups
	}
	group, found := podGroups[ownerRef.UID]
	if !found {
		return podGroups
	}
	if group.sample == nil && pod.Pod.Spec.NodeName == "" {
		group.sample = pod
		group.ownerUid = ownerRef.UID
	}
	group.podCount += 1
	podGroups[ownerRef.UID] = group
	return podGroups
}

func makePodGroup(desiredReplicas int) podGroup {
	return podGroup{
		podCount:        0,
		desiredReplicas: desiredReplicas,
	}
}
