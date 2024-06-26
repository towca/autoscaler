package dynamicresources

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/api/resource/v1alpha2"
)

// NodeDynamicResources contains objects associated with a Node that define some of the Node's resources outside the
// Node object itself. Scheduler needs to evaluate both the Node and the associated objects to decide if certain pods
// can be scheduled. Because of this, CA has to simulate the associated objects along the Node. All fields are optional,
// nothing set means that there aren't any associated objects.
type NodeDynamicResources struct {
	ResourceSlicesV1Alpha2 []*v1alpha2.ResourceSlice
}

// PodDynamicResourceRequests contains objects associated with a Pod that define some of the Pod's resource requests
// outside the Pod object itself. Scheduler needs this information for scheduled pods to know how much of
// the dynamic resources are reserved on the Pod's Node during subsequent scheduling attempts. Scheduler also needs this
// information for pending pods to evaluate whether a given Node has enough resources to satisfy the requests. All fields are
// optional, nothing set means that there aren't any associated objects.
type PodDynamicResourceRequests struct {
	ResourceClaimsV1Alpha2          []*v1alpha2.ResourceClaim
	ResourceClaimParametersV1Alpha2 []*v1alpha2.ResourceClaimParameters
}

type snapshotV1alpha2 struct {
	resourceClaims          []*v1alpha2.ResourceClaim
	resourceClaimParameters []*v1alpha2.ResourceClaimParameters
	resourceSlices          []*v1alpha2.ResourceSlice
}

// Snapshot contains a point-in-time view of all DRA-related objects that CA potentially needs to simulate.
type Snapshot struct {
	snapshotV1a2 snapshotV1alpha2
}

func (s Snapshot) PodResourceRequests(pod *apiv1.Pod) PodDynamicResourceRequests {
	// TODO(DRA): Find claims and params for the pod in the snapshot, fill in.
	return PodDynamicResourceRequests{
		ResourceClaimsV1Alpha2:          nil,
		ResourceClaimParametersV1Alpha2: nil,
	}
}

func (s Snapshot) NodeResources(node *apiv1.Node) NodeDynamicResources {
	// TODO(DRA): Find slices for the node in the snapshot, fill in.
	return NodeDynamicResources{
		ResourceSlicesV1Alpha2: nil,
	}
}
