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
	"errors"
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1alpha3"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/autoscaler/cluster-autoscaler/dynamicresources"
	"k8s.io/klog/v2"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

// NodeResourceInfo contains all information about a Node and its associated resources needed by the scheduler.
type NodeResourceInfo struct {
	*apiv1.Node
	DynamicResources schedulerframework.NodeDynamicResources
}

// PodResourceInfo contains all information about a Pod and its associated resource requests needed by the scheduler.
type PodResourceInfo struct {
	*apiv1.Pod
	DynamicResourceRequests schedulerframework.PodDynamicResourceRequests
}

func (p *PodResourceInfo) DeepCopy() *PodResourceInfo {
	return &PodResourceInfo{
		Pod:                     p.Pod.DeepCopy(),
		DynamicResourceRequests: p.DynamicResourceRequests.DeepCopy(),
	}
}

func (p *PodResourceInfo) AllocateClaims(allocatedClaims map[types.UID]*resourceapi.ResourceClaim) (*PodResourceInfo, error) {
	result := p.DeepCopy()
	allocated := 0
	for i, claim := range result.DynamicResourceRequests.ResourceClaims {
		if claim, found := allocatedClaims[claim.UID]; found {
			err := dynamicresources.AddPodReservationIfNeededInPlace(claim, p.Pod)
			if err != nil {
				return nil, err
			}
			result.DynamicResourceRequests.ResourceClaims[i] = claim
			allocated++
		}
	}
	if allocated != len(allocatedClaims) {
		return nil, fmt.Errorf("some claims not found in the pod")
	}
	return result, nil
}

// NewNodeResourceInfo combines a node with its associated DRA objects.
func NewNodeResourceInfo(node *apiv1.Node, draObjects dynamicresources.Snapshot) *NodeResourceInfo {
	return &NodeResourceInfo{Node: node, DynamicResources: draObjects.NodeResources(node)}
}

// NewPodResourceInfo combines a pod with its associated DRA objects.
func NewPodResourceInfo(pod *apiv1.Pod, draObjects dynamicresources.Snapshot) *PodResourceInfo {
	return &PodResourceInfo{Pod: pod, DynamicResourceRequests: draObjects.PodResourceRequests(pod)}
}

// ResourceInfos translates a NodeInfo into a NodeResourceInfo and a list of PodResourceInfos.
func ResourceInfos(nodeInfo *schedulerframework.NodeInfo) (*NodeResourceInfo, []*PodResourceInfo) {
	var podInfos []*PodResourceInfo
	for _, p := range nodeInfo.Pods {
		podInfos = append(podInfos, &PodResourceInfo{Pod: p.Pod, DynamicResourceRequests: p.DynamicResourceRequests})
	}
	return &NodeResourceInfo{Node: nodeInfo.Node(), DynamicResources: nodeInfo.DynamicResources()}, podInfos
}

// NewNodeInfo creates a new NodeInfo based on a Node and its Pods, as well as any associated DRA objects.
func NewNodeInfo(node *NodeResourceInfo, pods []*PodResourceInfo) *schedulerframework.NodeInfo {
	result := schedulerframework.NewNodeInfo()
	result.SetNodeWithDynamicResources(node.Node, node.DynamicResources)
	for _, pod := range pods {
		result.AddPodWithDynamicRequests(pod.Pod, pod.DynamicResourceRequests)
	}
	return result
}

// ClusterSnapshot is abstraction of cluster state used for predicate simulations.
// It exposes mutation methods and can be viewed as scheduler's SharedLister.
type ClusterSnapshot interface {
	schedulerframework.SharedLister
	schedulerframework.SharedDraManager
	// AddNode adds node to the snapshot.
	AddNode(node *NodeResourceInfo) error
	// AddNodes adds nodes to the snapshot.
	AddNodes(nodes []*NodeResourceInfo) error
	// RemoveNode removes a Node (as well as all associated info like its pods and dynamic resources) from the snapshot.
	RemoveNode(nodeName string) error
	// AddPod adds pod to the snapshot and schedules it to given node.
	AddPod(pod *PodResourceInfo, nodeName string) error
	// RemovePod removes a pod (as well as all associated info like its dynamic resource requests) from the snapshot.
	RemovePod(namespace string, podName string, nodeName string) (*PodResourceInfo, error)
	// AddNodeWithPods adds a node and set of pods to be scheduled to this node to the snapshot.
	AddNodeWithPods(node *NodeResourceInfo, pods []*PodResourceInfo) error
	// IsPVCUsedByPods returns if the pvc is used by any pod, key = <namespace>/<pvc_name>
	IsPVCUsedByPods(key string) bool

	SetGlobalResourceSlices(slices []*resourceapi.ResourceSlice)
	SetAllResourceClaims(claims []*resourceapi.ResourceClaim)

	GetResourceClaimAllocations() map[types.UID]*resourceapi.ResourceClaim
	ClearResourceClaimAllocations()

	SetAllDeviceClasses(classes []*resourceapi.DeviceClass)

	// Fork creates a fork of snapshot state. All modifications can later be reverted to moment of forking via Revert().
	// Use WithForkedSnapshot() helper function instead if possible.
	Fork()
	// Revert reverts snapshot state to moment of forking.
	Revert()
	// Commit commits changes done after forking.
	Commit() error
	// Clear reset cluster snapshot to empty, unforked state.
	Clear()
}

// Handle groups together everything needed to use the snapshot.
type Handle struct {
	ClusterSnapshot
	// DraObjectsSource should hold a snapshot of all DRA-related objects taken at the beginning of the loop (at the same time when pods
	// and nodes are snapshot). It's needed whenever _real_ (as opposed to the ones we fake in-memory) nodes and pods are added to the
	// snapshot, so that their DRA objects are added as well.
	DraObjectsSource dynamicresources.Snapshot
}

// ErrNodeNotFound means that a node wasn't found in the snapshot.
var ErrNodeNotFound = errors.New("node not found")

// WithForkedSnapshot is a helper function for snapshot that makes sure all Fork() calls are closed with Commit() or Revert() calls.
// The function return (error, error) pair. The first error comes from the passed function, the second error indicate the success of the function itself.
func WithForkedSnapshot(snapshot ClusterSnapshot, f func() (bool, error)) (error, error) {
	var commit bool
	var err, cleanupErr error
	snapshot.Fork()
	defer func() {
		if commit {
			cleanupErr = snapshot.Commit()
			if cleanupErr != nil {
				klog.Errorf("Got error when calling ClusterSnapshot.Commit(), will try to revert; %v", cleanupErr)
			}
		}
		if !commit || cleanupErr != nil {
			snapshot.Revert()
		}
	}()
	commit, err = f()
	return err, cleanupErr
}
