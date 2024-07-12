/*
Copyright 2022 The Kubernetes Authors.

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

package podlistprocessor

import (
	"k8s.io/autoscaler/cluster-autoscaler/context"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/clustersnapshot"
	pod_util "k8s.io/autoscaler/cluster-autoscaler/utils/pod"
	"k8s.io/klog/v2"
)

type currentlyDrainedNodesPodListProcessor struct {
}

// NewCurrentlyDrainedNodesPodListProcessor returns a new processor adding pods
// from currently drained nodes to the unschedulable pods.
func NewCurrentlyDrainedNodesPodListProcessor() *currentlyDrainedNodesPodListProcessor {
	return &currentlyDrainedNodesPodListProcessor{}
}

// Process adds recreatable pods from currently drained nodes
func (p *currentlyDrainedNodesPodListProcessor) Process(context *context.AutoscalingContext, unschedulablePods []*clustersnapshot.PodResourceInfo) ([]*clustersnapshot.PodResourceInfo, error) {
	drainedPods := currentlyDrainedPods(context)
	recreatableDrainedPods := clustersnapshot.FilterByPod(drainedPods, pod_util.IsRecreatablePod)
	clearedRecreatableDrainedPods := clustersnapshot.ApplyToPods(recreatableDrainedPods, pod_util.ClearPodName)
	return append(unschedulablePods, clearedRecreatableDrainedPods...), nil
}

func (p *currentlyDrainedNodesPodListProcessor) CleanUp() {
}

func currentlyDrainedPods(context *context.AutoscalingContext) []*clustersnapshot.PodResourceInfo {
	var pods []*clustersnapshot.PodResourceInfo
	_, nodeNames := context.ScaleDownActuator.CheckStatus().DeletionsInProgress()
	for _, nodeName := range nodeNames {
		nodeInfo, err := context.ClusterSnapshot.NodeInfos().Get(nodeName)
		if err != nil {
			klog.Warningf("Couldn't get node %v info, assuming the node got deleted already: %v", nodeName, err)
			continue
		}
		for _, podInfo := range nodeInfo.Pods {
			// Filter out pods that has deletion timestamp set
			if podInfo.Pod.DeletionTimestamp != nil {
				klog.Infof("Pod %v has deletion timestamp set, skipping injection to unschedulable pods list", podInfo.Pod.Name)
				continue
			}
			pods = append(pods, &clustersnapshot.PodResourceInfo{Pod: podInfo.Pod, DynamicResourceRequests: podInfo.DynamicResourceRequests})
		}
	}
	return pods
}
