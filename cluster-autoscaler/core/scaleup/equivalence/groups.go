/*
Copyright 2019 The Kubernetes Authors.

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

package equivalence

import (
	"k8s.io/autoscaler/cluster-autoscaler/simulator/clustersnapshot"
	"reflect"

	"k8s.io/autoscaler/cluster-autoscaler/utils"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/autoscaler/cluster-autoscaler/processors/status"
	"k8s.io/autoscaler/cluster-autoscaler/utils/drain"
	pod_utils "k8s.io/autoscaler/cluster-autoscaler/utils/pod"
)

// PodGroup contains a group of pods that are equivalent in terms of schedulability.
type PodGroup struct {
	Pods              []*clustersnapshot.PodResourceInfo
	SchedulingErrors  map[string]status.Reasons
	SchedulableGroups []string
	Schedulable       bool
}

// BuildPodGroups prepares pod groups with equivalent scheduling properties.
func BuildPodGroups(pods []*clustersnapshot.PodResourceInfo) []*PodGroup {
	podEquivalenceGroups := []*PodGroup{}
	for _, pods := range groupPodsBySchedulingProperties(pods) {
		podEquivalenceGroups = append(podEquivalenceGroups, &PodGroup{
			Pods:             pods,
			SchedulingErrors: map[string]status.Reasons{},
			Schedulable:      false,
		})
	}
	return podEquivalenceGroups
}

type equivalenceGroupId int
type equivalenceGroup struct {
	id           equivalenceGroupId
	representant *clustersnapshot.PodResourceInfo
}

const maxEquivalenceGroupsByController = 10

// groupPodsBySchedulingProperties groups pods based on scheduling properties. Group ID is meaningless.
// TODO(x13n): refactor this to have shared logic with PodSchedulableMap.
func groupPodsBySchedulingProperties(pods []*clustersnapshot.PodResourceInfo) map[equivalenceGroupId][]*clustersnapshot.PodResourceInfo {
	podEquivalenceGroups := map[equivalenceGroupId][]*clustersnapshot.PodResourceInfo{}
	equivalenceGroupsByController := make(map[types.UID][]equivalenceGroup)

	var nextGroupId equivalenceGroupId
	for _, pod := range pods {
		controllerRef := drain.ControllerRef(pod.Pod)
		if controllerRef == nil || pod_utils.IsDaemonSetPod(pod.Pod) {
			podEquivalenceGroups[nextGroupId] = []*clustersnapshot.PodResourceInfo{pod}
			nextGroupId++
			continue
		}

		egs := equivalenceGroupsByController[controllerRef.UID]
		if gid := match(egs, pod.Pod); gid != nil {
			podEquivalenceGroups[*gid] = append(podEquivalenceGroups[*gid], pod)
			continue
		}
		if len(egs) < maxEquivalenceGroupsByController {
			// Avoid too many different pods per owner reference.
			newGroup := equivalenceGroup{
				id:           nextGroupId,
				representant: pod,
			}
			equivalenceGroupsByController[controllerRef.UID] = append(egs, newGroup)
		}
		podEquivalenceGroups[nextGroupId] = append(podEquivalenceGroups[nextGroupId], pod)
		nextGroupId++
	}

	return podEquivalenceGroups
}

// match tries to find an equivalence group for a given pod and returns the
// group id or nil if the group can't be found.
func match(egs []equivalenceGroup, pod *apiv1.Pod) *equivalenceGroupId {
	for _, g := range egs {
		if reflect.DeepEqual(pod.Labels, g.representant.Labels) && utils.PodSpecSemanticallyEqual(pod.Spec, g.representant.Spec) {
			return &g.id
		}
	}
	return nil
}
