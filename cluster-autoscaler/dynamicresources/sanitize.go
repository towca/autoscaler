package dynamicresources

import (
	"fmt"

	resourceapi "k8s.io/api/resource/v1alpha3"
	"k8s.io/apimachinery/pkg/util/uuid"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

// SanitizedNodeDynamicResources returns a deep copy of the provided NodeDynamicResources where:
// - NodeName pointers in all DRA objects are updated to the provided nodeName.
// - Names of all DRA objects get the provided nameSuffix appended.
// - UIDs of all DRA objects are randomized.
// - Names of all Pools that the slices belong to get the provided nameSuffix appended.
//
// This needs to be done anytime we want to add a "copy" of some Node (and so also of its NodeDynamicResources)
// to ClusterSnapshot.
func SanitizedNodeDynamicResources(ndr schedulerframework.NodeDynamicResources, nodeName, nameSuffix string) schedulerframework.NodeDynamicResources {
	sanitizedNdr := ndr.DeepCopy()
	for _, slice := range sanitizedNdr.ResourceSlices {
		slice.Name = fmt.Sprintf("%s-%s", slice.Name, nameSuffix)
		slice.Spec.Pool.Name = fmt.Sprintf("%s-%s", slice.Spec.Pool.Name, nameSuffix)
		slice.UID = uuid.NewUUID()
		slice.Spec.NodeName = nodeName
	}
	return sanitizedNdr
}

// SanitizedPodDynamicResourceRequests returns a deep copy of the provided PodDynamicResourceRequests where:
//   - Names of all DRA objects get the provided nameSuffix appended.
//   - UIDs of all DRA objects are randomized.
//   - Names of all Pools in the allocation results get the provided nameSuffix appended. The suffix should match
//     the suffix used while sanitizing Pools in ResourceSlices of its Node.
//
// This needs to be done anytime we want to add a "copy" of some Pod (and so also of its PodDynamicResourceRequests)
// to ClusterSnapshot.
func SanitizedPodDynamicResourceRequests(pdr schedulerframework.PodDynamicResourceRequests, nameSuffix string) schedulerframework.PodDynamicResourceRequests {
	sanitizedPdr := pdr.DeepCopy()
	for _, claim := range sanitizedPdr.ResourceClaims {
		claim.Name = fmt.Sprintf("%s-%s", claim.Name, nameSuffix)
		claim.UID = uuid.NewUUID()

		var sanitizedAllocations []resourceapi.DeviceRequestAllocationResult
		for _, devAlloc := range claim.Status.Allocation.Devices.Results {
			devAlloc.Pool = fmt.Sprintf("%s-%s", devAlloc.Pool, nameSuffix)
			sanitizedAllocations = append(sanitizedAllocations, devAlloc)
		}

		claim.Status.Allocation.Devices.Results = sanitizedAllocations
	}
	return sanitizedPdr
}
