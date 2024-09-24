package dynamicresources

import (
	"fmt"
	apiv1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1alpha3"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/utils/ptr"
)

func ClaimOwningPod(claim *resourceapi.ResourceClaim) (string, types.UID) {
	for _, owner := range claim.OwnerReferences {
		if ptr.Deref(owner.Controller, false) &&
			owner.APIVersion == "v1" &&
			owner.Kind == "Pod" {
			return owner.Name, owner.UID
		}
	}
	return "", ""
}

func ClaimAllocated(claim *resourceapi.ResourceClaim) bool {
	return claim.Status.Allocation != nil
}

func SameAllocation(claimA, claimB *resourceapi.ResourceClaim) bool {
	return apiequality.Semantic.DeepEqual(claimA.Status.Allocation, claimB.Status.Allocation)
}

func SplitClaimsByOwnership(claims []*resourceapi.ResourceClaim) (podOwned, global []*resourceapi.ResourceClaim) {
	for _, claim := range claims {
		if podName, _ := ClaimOwningPod(claim); podName != "" {
			podOwned = append(podOwned, claim)
		} else {
			global = append(global, claim)
		}
	}
	return podOwned, global
}

func ClearPodReservationInPlace(claim *resourceapi.ResourceClaim, pod *apiv1.Pod) {
	newReservedFor := make([]resourceapi.ResourceClaimConsumerReference, 0, len(claim.Status.ReservedFor))
	for _, consumerRef := range claim.Status.ReservedFor {
		if ClaimConsumerReferenceMatchesPod(pod, consumerRef) {
			continue
		}
		newReservedFor = append(newReservedFor, consumerRef)
	}
	claim.Status.ReservedFor = newReservedFor
}

func AddPodReservationIfNeededInPlace(claim *resourceapi.ResourceClaim, pod *apiv1.Pod) error {
	alreadyReservedForPod := false
	for _, consumerRef := range claim.Status.ReservedFor {
		if ClaimConsumerReferenceMatchesPod(pod, consumerRef) {
			alreadyReservedForPod = true
			break
		}
	}
	if !alreadyReservedForPod {
		if len(claim.Status.ReservedFor) >= resourceapi.ResourceClaimReservedForMaxSize {
			return fmt.Errorf("claim already reserved for %d consumers, can't add more", len(claim.Status.ReservedFor))
		}
		claim.Status.ReservedFor = append(claim.Status.ReservedFor, PodClaimConsumerReference(pod))
	}
	return nil
}

func ClaimInUse(claim *resourceapi.ResourceClaim) bool {
	return len(claim.Status.ReservedFor) > 0
}

func DeallocateClaimInPlace(claim *resourceapi.ResourceClaim) {
	claim.Status.Allocation = nil
}

func ClaimConsumerReferenceMatchesPod(pod *apiv1.Pod, ref resourceapi.ResourceClaimConsumerReference) bool {
	return ref.APIGroup == "" && ref.Resource == "pods" && ref.Name == pod.Name && ref.UID == pod.UID
}
func PodClaimConsumerReference(pod *apiv1.Pod) resourceapi.ResourceClaimConsumerReference {
	return resourceapi.ResourceClaimConsumerReference{
		Name:     pod.Name,
		UID:      pod.UID,
		Resource: "pods",
		APIGroup: "",
	}
}

func NodeInfoResourceClaims(nodeInfo *schedulerframework.NodeInfo) []*resourceapi.ResourceClaim {
	processedClaims := map[types.UID]bool{}
	var result []*resourceapi.ResourceClaim
	for _, pod := range nodeInfo.Pods {
		for _, claim := range pod.DynamicResourceRequests.ResourceClaims {
			if processedClaims[claim.UID] {
				// Shared claim, already grouped.
				continue
			}
			result = append(result, claim)
			processedClaims[claim.UID] = true
		}
	}
	return result
}
func GroupAllocatedDevices(claims []*resourceapi.ResourceClaim) map[string]map[string][]string {
	result := map[string]map[string][]string{}
	for _, claim := range claims {
		alloc := claim.Status.Allocation
		if alloc == nil {
			klog.Warningf("Shouldn't happeeeeeeeen")
			continue
		}

		for _, deviceAlloc := range alloc.Devices.Results {
			if result[deviceAlloc.Driver] == nil {
				result[deviceAlloc.Driver] = map[string][]string{}
			}
			result[deviceAlloc.Driver][deviceAlloc.Pool] = append(result[deviceAlloc.Driver][deviceAlloc.Pool], deviceAlloc.Device)
		}
	}
	return result
}

func GetAllDevices(slices []*resourceapi.ResourceSlice) []resourceapi.Device {
	var devices []resourceapi.Device
	for _, slice := range slices {
		devices = append(devices, slice.Spec.Devices...)
	}
	return devices
}

func GroupSlices(slices []*resourceapi.ResourceSlice) map[string]map[string][]*resourceapi.ResourceSlice {
	result := map[string]map[string][]*resourceapi.ResourceSlice{}
	for _, slice := range slices {
		driver := slice.Spec.Driver
		pool := slice.Spec.Pool.Name
		if result[driver] == nil {
			result[driver] = map[string][]*resourceapi.ResourceSlice{}
		}
		result[driver][pool] = append(result[driver][pool], slice)
	}
	return result
}

func AllCurrentGenSlices(slices []*resourceapi.ResourceSlice) ([]*resourceapi.ResourceSlice, error) {
	var maxGenSlices []*resourceapi.ResourceSlice
	maxGen := int64(0)
	for _, slice := range slices {
		gen := slice.Spec.Pool.Generation
		if gen > maxGen {
			maxGen = gen
			maxGenSlices = []*resourceapi.ResourceSlice{slice}
			continue
		}
		if gen == maxGen {
			maxGenSlices = append(maxGenSlices, slice)
		}
	}

	foundCurrentSlices := len(maxGenSlices)
	if foundCurrentSlices == 0 {
		return nil, nil
	}

	if wantCurrentSlices := maxGenSlices[0].Spec.Pool.ResourceSliceCount; int64(foundCurrentSlices) != wantCurrentSlices {
		return nil, fmt.Errorf("newest generation: %d, slice count: %d - found only %d slices", maxGen, wantCurrentSlices, foundCurrentSlices)
	}

	return maxGenSlices, nil
}
