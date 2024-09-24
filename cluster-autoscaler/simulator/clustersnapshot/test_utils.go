/*
Copyright 2020 The Kubernetes Authors.

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

package clustersnapshot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
)

// InitializeClusterSnapshotOrDie clears cluster snapshot and then initializes it with given set of nodes and pods.
// Both Spec.NodeName and Status.NominatedNodeName are used when simulating scheduling pods.
func InitializeClusterSnapshotOrDie(t *testing.T, snapshot ClusterSnapshot, nodes []*apiv1.Node, pods []*apiv1.Pod) {
	var nodeRes []*NodeResourceInfo
	for _, node := range nodes {
		nodeRes = append(nodeRes, &NodeResourceInfo{Node: node})
	}
	var podRes []*PodResourceInfo
	for _, pod := range pods {
		podRes = append(podRes, &PodResourceInfo{Pod: pod})
	}
	InitializeClusterSnapshotWithDynamicResourcesOrDie(t, snapshot, nodeRes, podRes)
}

func InitializeClusterSnapshotWithDynamicResourcesOrDie(t *testing.T, snapshot ClusterSnapshot, nodes []*NodeResourceInfo, pods []*PodResourceInfo) {
	var err error

	snapshot.Clear()

	for _, node := range nodes {
		err = snapshot.AddNode(node)
		assert.NoError(t, err, "error while adding node %s", node.Node.Name)
	}

	for _, pod := range pods {
		if pod.Pod.Spec.NodeName != "" {
			err = snapshot.AddPod(pod, pod.Pod.Spec.NodeName)
			assert.NoError(t, err, "error while adding pod %s/%s to node %s", pod.Pod.Namespace, pod.Pod.Name, pod.Pod.Spec.NodeName)
		} else if pod.Pod.Status.NominatedNodeName != "" {
			err = snapshot.AddPod(pod, pod.Pod.Status.NominatedNodeName)
			assert.NoError(t, err, "error while adding pod %s/%s to nominated node %s", pod.Pod.Namespace, pod.Pod.Name, pod.Pod.Status.NominatedNodeName)
		} else {
			assert.Failf(t, "", "pod %s/%s does not have Spec.NodeName nor Status.NominatedNodeName set", pod.Pod.Namespace, pod.Pod.Name)
		}
	}
}

func WrapPodsInResourceInfos(pods []*apiv1.Pod) []*PodResourceInfo {
	var result []*PodResourceInfo
	for _, pod := range pods {
		result = append(result, &PodResourceInfo{Pod: pod})
	}
	return result
}

func WrapNodesInResourceInfos(nodes []*apiv1.Node) []*NodeResourceInfo {
	var result []*NodeResourceInfo
	for _, node := range nodes {
		result = append(result, &NodeResourceInfo{Node: node})
	}
	return result
}
