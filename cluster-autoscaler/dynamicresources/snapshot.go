package dynamicresources

import (
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1alpha2"
	"k8s.io/klog/v2"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

type objectRef struct {
	name      string
	namespace string
}

// Snapshot contains a point-in-time view of all DRA-related objects that CA potentially needs to simulate.
type Snapshot struct {
	resourceClaimsByRef          map[objectRef]*resourceapi.ResourceClaim
	resourceClaimParametersByRef map[objectRef]*resourceapi.ResourceClaimParameters
	resourceSlicesByNodeName     map[string][]*resourceapi.ResourceSlice
}

func (s Snapshot) PodResourceRequests(pod *apiv1.Pod) schedulerframework.PodDynamicResourceRequests {
	result := schedulerframework.PodDynamicResourceRequests{}

	for _, claimRef := range pod.Spec.ResourceClaims {
		claim, claimParams, err := s.claimAndParams(pod, claimRef)
		if err != nil {
			klog.Warningf("DRA: pod %s/%s, claim ref %q: error while determining DRA objects: %s", pod.Namespace, pod.Name, claimRef.Name, err)
			continue
		}
		result.ResourceClaims = append(result.ResourceClaims, claim)
		if claimParams != nil {
			result.ResourceClaimParameters = append(result.ResourceClaimParameters, claimParams)
		}
	}

	return result
}

func (s Snapshot) NodeResources(node *apiv1.Node) schedulerframework.NodeDynamicResources {
	return schedulerframework.NodeDynamicResources{
		ResourceSlices: s.resourceSlicesByNodeName[node.Name],
	}
}

func (s Snapshot) claimAndParams(pod *apiv1.Pod, claimRef apiv1.PodResourceClaim) (*resourceapi.ResourceClaim, *resourceapi.ResourceClaimParameters, error) {
	claimName := claimRefToName(pod, claimRef)
	if claimName == "" {
		return nil, nil, fmt.Errorf("couldn't determine ResourceClaim name")
	}

	claim, found := s.resourceClaimsByRef[objectRef{name: claimName, namespace: pod.Namespace}]
	if !found {
		return nil, nil, fmt.Errorf("couldn't find ResourceClaim %q", claimName)
	}

	paramsRef := claim.Spec.ParametersRef
	if paramsRef == nil {
		return claim, nil, nil
	}

	if paramsRef.APIGroup != resourceapi.GroupName || paramsRef.Kind != "ResourceClaimParameters" {
		return nil, nil, fmt.Errorf("parametersRef refers to something other than resource.k8s.io/ResourceClaimParameters")
	}
	params, found := s.resourceClaimParametersByRef[objectRef{name: paramsRef.Name, namespace: pod.Namespace}]
	if !found {
		return nil, nil, fmt.Errorf("couldn't find ResourceClaimParameters %q", paramsRef.Name)
	}

	return claim, params, nil
}

func claimRefToName(pod *apiv1.Pod, claimRef apiv1.PodResourceClaim) string {
	if claimRef.Source.ResourceClaimName != nil {
		return *claimRef.Source.ResourceClaimName
	}
	for _, claimStatus := range pod.Status.ResourceClaimStatuses {
		if claimStatus.Name == claimRef.Name && claimStatus.ResourceClaimName != nil {
			return *claimStatus.ResourceClaimName
		}
	}
	return ""
}
