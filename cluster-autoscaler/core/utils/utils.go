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

package utils

import (
	"fmt"
	"math/rand"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/dynamicresources"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/clustersnapshot"
	"k8s.io/autoscaler/cluster-autoscaler/utils/daemonset"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	"k8s.io/autoscaler/cluster-autoscaler/utils/gpu"
	"k8s.io/autoscaler/cluster-autoscaler/utils/labels"
	"k8s.io/autoscaler/cluster-autoscaler/utils/taints"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

// GetNodeInfoFromTemplate returns NodeInfo object built base on TemplateNodeInfo returned by NodeGroup.TemplateNodeInfo().
func GetNodeInfoFromTemplate(nodeGroup cloudprovider.NodeGroup, daemonsets []*appsv1.DaemonSet, taintConfig taints.TaintConfig) (*schedulerframework.NodeInfo, errors.AutoscalerError) {
	id := nodeGroup.Id()
	baseNodeInfo, err := nodeGroup.TemplateNodeInfo()
	if err != nil {
		return nil, errors.ToAutoscalerError(errors.CloudProviderError, err)
	}

	labels.UpdateDeprecatedLabels(baseNodeInfo.Node().ObjectMeta.Labels)

	randSuffix := fmt.Sprintf("%d", rand.Int63())

	// Deep copy and sanitize the template node and the associated DRA objects returned from the cloud provider
	sanitizedNode, typedErr := SanitizeNode(clustersnapshot.NodeResourceInfo{Node: baseNodeInfo.Node(), DynamicResources: baseNodeInfo.DynamicResources()}, id, taintConfig, randSuffix)
	if err != nil {
		return nil, typedErr
	}

	var startupPods []clustersnapshot.PodResourceInfo
	// Determine DS pods that will be scheduled on the Node created from this template
	dsPods, err := daemonset.GetDaemonSetPodsForNode(sanitizedNode.Node, daemonsets)
	if err != nil {
		return nil, errors.ToAutoscalerError(errors.InternalError, err)
	}
	for _, pod := range dsPods {
		// TODO(DRA): Simulate DRA requests for simulated DS pods.
		startupPods = append(startupPods, clustersnapshot.PodResourceInfo{Pod: pod, DynamicResourceRequests: schedulerframework.PodDynamicResourceRequests{}})
	}
	// Add the startup pods included in the node info returned from the cloud provider
	for _, podInfo := range baseNodeInfo.Pods {
		startupPods = append(startupPods, clustersnapshot.PodResourceInfo{Pod: podInfo.Pod, DynamicResourceRequests: podInfo.DynamicResourceRequests})
	}
	// Deep copy and sanitize the startup Pods and the associated DRA objects into fakes pointing to the fake sanitized Node
	sanitizedStartupPods := SanitizePods(startupPods, sanitizedNode.Node.Name, randSuffix)

	// Build the final node info with all 3 parts (Node, Pods, DRA objects) sanitized and in sync.
	return clustersnapshot.NewNodeInfo(sanitizedNode, sanitizedStartupPods), nil
}

// isVirtualNode determines if the node is created by virtual kubelet
func isVirtualNode(node *apiv1.Node) bool {
	return node.ObjectMeta.Labels["type"] == "virtual-kubelet"
}

// FilterOutNodesFromNotAutoscaledGroups return subset of input nodes for which cloud provider does not
// return autoscaled node group.
func FilterOutNodesFromNotAutoscaledGroups(nodes []*apiv1.Node, cloudProvider cloudprovider.CloudProvider) ([]*apiv1.Node, errors.AutoscalerError) {
	result := make([]*apiv1.Node, 0)

	for _, node := range nodes {
		// Exclude the virtual node here since it may have lots of resource and exceed the total resource limit
		if isVirtualNode(node) {
			continue
		}
		nodeGroup, err := cloudProvider.NodeGroupForNode(node)
		if err != nil {
			return []*apiv1.Node{}, errors.ToAutoscalerError(errors.CloudProviderError, err)
		}
		if nodeGroup == nil || reflect.ValueOf(nodeGroup).IsNil() {
			result = append(result, node)
		}
	}
	return result, nil
}

// DeepCopyNodeInfo clones the provided nodeInfo
func DeepCopyNodeInfo(nodeInfo *schedulerframework.NodeInfo) *schedulerframework.NodeInfo {
	newPods := make([]clustersnapshot.PodResourceInfo, 0)
	for _, podInfo := range nodeInfo.Pods {
		newPods = append(newPods, clustersnapshot.PodResourceInfo{Pod: podInfo.Pod.DeepCopy(), DynamicResourceRequests: podInfo.DynamicResourceRequests.DeepCopy()})
	}
	newNode := clustersnapshot.NodeResourceInfo{Node: nodeInfo.Node().DeepCopy(), DynamicResources: nodeInfo.DynamicResources().DeepCopy()}
	return clustersnapshot.NewNodeInfo(newNode, newPods)
}

// SanitizeNode cleans up nodes used for node group templates
func SanitizeNode(nodeResInfo clustersnapshot.NodeResourceInfo, nodeGroup string, taintConfig taints.TaintConfig, nameSuffix string) (clustersnapshot.NodeResourceInfo, errors.AutoscalerError) {
	newNode := nodeResInfo.Node.DeepCopy()
	nodeName := fmt.Sprintf("template-node-for-%s-%s", nodeGroup, nameSuffix)
	newNode.Labels = make(map[string]string, len(nodeResInfo.Node.Labels))
	for k, v := range nodeResInfo.Node.Labels {
		if k != apiv1.LabelHostname {
			newNode.Labels[k] = v
		} else {
			newNode.Labels[k] = nodeName
		}
	}
	newNode.Name = nodeName
	newNode.Spec.Taints = taints.SanitizeTaints(newNode.Spec.Taints, taintConfig)
	newDynamicResources := dynamicresources.SanitizedNodeDynamicResources(nodeResInfo.DynamicResources, newNode.Name, nameSuffix)
	return clustersnapshot.NodeResourceInfo{Node: newNode, DynamicResources: newDynamicResources}, nil
}

// SanitizePods sanitizes a collection of Pods - see comment on SanitizePod.
func SanitizePods(pods []clustersnapshot.PodResourceInfo, nodeName string, nameSuffix string) []clustersnapshot.PodResourceInfo {
	var result []clustersnapshot.PodResourceInfo
	for _, pod := range pods {
		result = append(result, SanitizePod(pod, nodeName, nameSuffix))
	}
	return result
}

// SanitizePod returns a copy of the provided Pod meant to be injected into the cluster snapshot along the original pod (so
// the name and UID have to be changed), scheduled on a new Node.
func SanitizePod(podResInfo clustersnapshot.PodResourceInfo, nodeName string, nameSuffix string) clustersnapshot.PodResourceInfo {
	podCopy := podResInfo.Pod.DeepCopy()
	podCopy.Name = fmt.Sprintf("%s-%s", podResInfo.Pod.Name, nameSuffix)
	podCopy.UID = uuid.NewUUID()
	podCopy.Spec.NodeName = nodeName
	dynamicRequestsCopy := dynamicresources.SanitizedPodDynamicResourceRequests(podResInfo.DynamicResourceRequests, nameSuffix)
	return clustersnapshot.PodResourceInfo{Pod: podCopy, DynamicResourceRequests: dynamicRequestsCopy}
}

func hasHardInterPodAffinity(affinity *apiv1.Affinity) bool {
	if affinity == nil {
		return false
	}
	if affinity.PodAffinity != nil {
		if len(affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
			return true
		}
	}
	if affinity.PodAntiAffinity != nil {
		if len(affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
			return true
		}
	}
	return false
}

// GetNodeCoresAndMemory extracts cpu and memory resources out of Node object
func GetNodeCoresAndMemory(node *apiv1.Node) (int64, int64) {
	cores := getNodeResource(node, apiv1.ResourceCPU)
	memory := getNodeResource(node, apiv1.ResourceMemory)
	return cores, memory
}

func getNodeResource(node *apiv1.Node, resource apiv1.ResourceName) int64 {
	nodeCapacity, found := node.Status.Capacity[resource]
	if !found {
		return 0
	}

	nodeCapacityValue := nodeCapacity.Value()
	if nodeCapacityValue < 0 {
		nodeCapacityValue = 0
	}

	return nodeCapacityValue
}

// GetOldestCreateTime returns oldest creation time out of the pods in the set
func GetOldestCreateTime(pods []*apiv1.Pod) time.Time {
	oldest := time.Now()
	for _, pod := range pods {
		if oldest.After(pod.CreationTimestamp.Time) {
			oldest = pod.CreationTimestamp.Time
		}
	}
	return oldest
}

// GetOldestCreateTimeWithGpu returns oldest creation time out of pods with GPU in the set
func GetOldestCreateTimeWithGpu(pods []*apiv1.Pod) (bool, time.Time) {
	oldest := time.Now()
	gpuFound := false
	for _, pod := range pods {
		if gpu.PodRequestsGpu(pod) {
			gpuFound = true
			if oldest.After(pod.CreationTimestamp.Time) {
				oldest = pod.CreationTimestamp.Time
			}
		}
	}
	return gpuFound, oldest
}
