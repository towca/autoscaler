package dynamicresources

import (
	apiv1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1alpha2"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

// Snapshot contains a point-in-time view of all DRA-related objects that CA potentially needs to simulate.
type Snapshot struct {
	resourceClaims          []*resourceapi.ResourceClaim
	resourceClaimParameters []*resourceapi.ResourceClaimParameters
	resourceSlices          []*resourceapi.ResourceSlice
}

func (s Snapshot) PodResourceRequests(pod *apiv1.Pod) schedulerframework.PodDynamicResourceRequests {
	// TODO(DRA): Find claims and params for the pod in the snapshot, fill in.
	return schedulerframework.PodDynamicResourceRequests{
		ResourceClaims:          nil,
		ResourceClaimParameters: nil,
	}
}

func (s Snapshot) NodeResources(node *apiv1.Node) schedulerframework.NodeDynamicResources {
	// TODO(DRA): Find slices for the node in the snapshot, fill in.
	return schedulerframework.NodeDynamicResources{
		ResourceSlices: nil,
	}
}
