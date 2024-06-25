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

	apiv1 "k8s.io/api/core/v1"
	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
	"k8s.io/klog/v2"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

// NodeDynamicResources contains objects associated with a Node that define some of the Node's resources outside the
// Node object itself. Scheduler needs to evaluate both the Node and the associated objects to decide if certain pods
// can be scheduled. Because of this, CA has to simulate the associated objects along the Node. All fields are optional,
// nothing set means that there aren't any associated objects.
type NodeDynamicResources struct {
	ResourceSlicesV1Alpha2 []*resourcev1alpha2.ResourceSlice
}

// NodeResourceInfo contains all information about a Node and its associated resources needed by the scheduler.
type NodeResourceInfo struct {
	Node             *apiv1.Node
	DynamicResources NodeDynamicResources
}

// PodDynamicResourceRequests contains objects associated with a Pod that define some of the Pod's resource requests
// outside the Pod object itself. Scheduler needs this information for scheduled pods to know how much of
// the dynamic resources are reserved on the Pod's Node during subsequent scheduling attempts. Scheduler also needs this
// information for pending pods to evaluate whether a given Node has enough resources to satisfy the requests. All fields are
// optional, nothing set means that there aren't any associated objects.
type PodDynamicResourceRequests struct {
	ResourceClaimsV1Alpha2          []*resourcev1alpha2.ResourceClaim
	ResourceClaimParametersV1Alpha2 []*resourcev1alpha2.ResourceClaimParameters
}

// PodResourceInfo contains all information about a Pod and its associated resource requests needed by the scheduler.
type PodResourceInfo struct {
	Pod                     *apiv1.Pod
	DynamicResourceRequests PodDynamicResourceRequests
}

// ClusterSnapshot is abstraction of cluster state used for predicate simulations.
// It exposes mutation methods and can be viewed as scheduler's SharedLister.
type ClusterSnapshot interface {
	schedulerframework.SharedLister
	// AddNode adds node to the snapshot.
	AddNode(node NodeResourceInfo) error
	// AddNodes adds nodes to the snapshot.
	AddNodes(nodes []NodeResourceInfo) error
	// RemoveNode removes a Node (as well as all associated info like its pods and dynamic resources) from the snapshot.
	RemoveNode(nodeName string) error
	// AddPod adds pod to the snapshot and schedules it to given node.
	AddPod(pod PodResourceInfo, nodeName string) error
	// RemovePod removes a pod (as well as all associated info like its dynamic resource requests) from the snapshot.
	RemovePod(namespace string, podName string, nodeName string) error
	// AddNodeWithPods adds a node and set of pods to be scheduled to this node to the snapshot.
	AddNodeWithPods(node NodeResourceInfo, pods []PodResourceInfo) error
	// IsPVCUsedByPods returns if the pvc is used by any pod, key = <namespace>/<pvc_name>
	IsPVCUsedByPods(key string) bool

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
