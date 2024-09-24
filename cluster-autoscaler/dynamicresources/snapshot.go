package dynamicresources

import (
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1alpha3"
	"k8s.io/klog/v2"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

type ResourceClaimRef struct {
	Name      string
	Namespace string
}

// Snapshot contains a point-in-time view of all DRA-related objects that CA potentially needs to simulate.
type Snapshot struct {
	resourceClaimsByRef        map[ResourceClaimRef]*resourceapi.ResourceClaim
	resourceSlicesByNodeName   map[string][]*resourceapi.ResourceSlice
	NonNodeLocalResourceSlices []*resourceapi.ResourceSlice
	DeviceClasses              []*resourceapi.DeviceClass
}

func (s Snapshot) PodResourceRequests(pod *apiv1.Pod) schedulerframework.PodDynamicResourceRequests {
	result := schedulerframework.PodDynamicResourceRequests{}

	for _, claimRef := range pod.Spec.ResourceClaims {
		claim, err := s.claimForPod(pod, claimRef)
		if err != nil {
			klog.Warningf("%s", s.resourceClaimsByRef)
			klog.Warningf("DRA: pod %s/%s, claim ref %q: error while determining DRA objects: %s", pod.Namespace, pod.Name, claimRef.Name, err)
			continue
		}
		result.ResourceClaims = append(result.ResourceClaims, claim)
	}

	return result
}

func (s Snapshot) NodeResources(node *apiv1.Node) schedulerframework.NodeDynamicResources {
	return schedulerframework.NodeDynamicResources{
		ResourceSlices: s.resourceSlicesByNodeName[node.Name],
	}
}

func (s Snapshot) AllResourceClaims() []*resourceapi.ResourceClaim {
	var result []*resourceapi.ResourceClaim
	for _, claim := range s.resourceClaimsByRef {
		result = append(result, claim)
	}
	return result
}

func (s Snapshot) claimForPod(pod *apiv1.Pod, claimRef apiv1.PodResourceClaim) (*resourceapi.ResourceClaim, error) {
	claimName := claimRefToName(pod, claimRef)
	if claimName == "" {
		return nil, fmt.Errorf("couldn't determine ResourceClaim name")
	}

	claim, found := s.resourceClaimsByRef[ResourceClaimRef{Name: claimName, Namespace: pod.Namespace}]
	if !found {
		return nil, fmt.Errorf("couldn't find ResourceClaim %q", claimName)
	}

	return claim, nil
}

func claimRefToName(pod *apiv1.Pod, claimRef apiv1.PodResourceClaim) string {
	if claimRef.ResourceClaimName != nil {
		return *claimRef.ResourceClaimName
	}
	for _, claimStatus := range pod.Status.ResourceClaimStatuses {
		if claimStatus.Name == claimRef.Name && claimStatus.ResourceClaimName != nil {
			return *claimStatus.ResourceClaimName
		}
	}
	return ""
}
