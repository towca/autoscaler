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
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1alpha3"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/autoscaler/cluster-autoscaler/dynamicresources"
	"k8s.io/klog/v2"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

// BasicClusterSnapshot is simple, reference implementation of ClusterSnapshot.
// It is inefficient. But hopefully bug-free and good for initial testing.
type BasicClusterSnapshot struct {
	data                     []*internalBasicSnapshotData
	globalResourceSlices     []*resourceapi.ResourceSlice
	allResourceClaims        map[dynamicresources.ResourceClaimRef]*resourceapi.ResourceClaim
	resourceClaimAllocations map[types.UID]*resourceapi.ResourceClaim
	allDeviceClasses         map[string]*resourceapi.DeviceClass
}

func (snapshot *BasicClusterSnapshot) GetResourceClaimAllocations() map[types.UID]*resourceapi.ResourceClaim {
	return snapshot.resourceClaimAllocations
}

func (snapshot *BasicClusterSnapshot) ClearResourceClaimAllocations() {
	snapshot.resourceClaimAllocations = map[types.UID]*resourceapi.ResourceClaim{}
}

func (snapshot *BasicClusterSnapshot) SetGlobalResourceSlices(slices []*resourceapi.ResourceSlice) {
	snapshot.globalResourceSlices = slices
}

func (snapshot *BasicClusterSnapshot) SetAllResourceClaims(claims []*resourceapi.ResourceClaim) {
	snapshot.allResourceClaims = map[dynamicresources.ResourceClaimRef]*resourceapi.ResourceClaim{}
	for _, claim := range claims {
		snapshot.allResourceClaims[dynamicresources.ResourceClaimRef{Name: claim.Name, Namespace: claim.Namespace}] = claim
	}
}

func (snapshot *BasicClusterSnapshot) SetAllDeviceClasses(classes []*resourceapi.DeviceClass) {
	snapshot.allDeviceClasses = map[string]*resourceapi.DeviceClass{}
	for _, class := range classes {
		snapshot.allDeviceClasses[class.Name] = class
	}
}

type internalBasicSnapshotData struct {
	nodeInfoMap          map[string]*schedulerframework.NodeInfo
	pvcNamespacePodMap   map[string]map[string]bool
	globalResourceClaims map[dynamicresources.ResourceClaimRef]*resourceapi.ResourceClaim
}

func (data *internalBasicSnapshotData) listNodeInfos() ([]*schedulerframework.NodeInfo, error) {
	nodeInfoList := make([]*schedulerframework.NodeInfo, 0, len(data.nodeInfoMap))
	for _, v := range data.nodeInfoMap {
		nodeInfoList = append(nodeInfoList, v)
	}
	return nodeInfoList, nil
}

func (data *internalBasicSnapshotData) listNodeInfosThatHavePodsWithAffinityList() ([]*schedulerframework.NodeInfo, error) {
	havePodsWithAffinityList := make([]*schedulerframework.NodeInfo, 0, len(data.nodeInfoMap))
	for _, v := range data.nodeInfoMap {
		if len(v.PodsWithAffinity) > 0 {
			havePodsWithAffinityList = append(havePodsWithAffinityList, v)
		}
	}

	return havePodsWithAffinityList, nil
}

func (data *internalBasicSnapshotData) listNodeInfosThatHavePodsWithRequiredAntiAffinityList() ([]*schedulerframework.NodeInfo, error) {
	havePodsWithRequiredAntiAffinityList := make([]*schedulerframework.NodeInfo, 0, len(data.nodeInfoMap))
	for _, v := range data.nodeInfoMap {
		if len(v.PodsWithRequiredAntiAffinity) > 0 {
			havePodsWithRequiredAntiAffinityList = append(havePodsWithRequiredAntiAffinityList, v)
		}
	}

	return havePodsWithRequiredAntiAffinityList, nil
}

func (data *internalBasicSnapshotData) getNodeInfo(nodeName string) (*schedulerframework.NodeInfo, error) {
	if v, ok := data.nodeInfoMap[nodeName]; ok {
		return v, nil
	}
	return nil, ErrNodeNotFound
}

func (data *internalBasicSnapshotData) isPVCUsedByPods(key string) bool {
	if v, found := data.pvcNamespacePodMap[key]; found && v != nil && len(v) > 0 {
		return true
	}
	return false
}

func (data *internalBasicSnapshotData) addPvcUsedByPod(pod *apiv1.Pod) {
	if pod == nil {
		return
	}
	nameSpace := pod.GetNamespace()
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		k := schedulerframework.GetNamespacedName(nameSpace, volume.PersistentVolumeClaim.ClaimName)
		_, found := data.pvcNamespacePodMap[k]
		if !found {
			data.pvcNamespacePodMap[k] = make(map[string]bool)
		}
		data.pvcNamespacePodMap[k][pod.GetName()] = true
	}
}

func (data *internalBasicSnapshotData) removePvcUsedByPod(pod *apiv1.Pod) {
	if pod == nil {
		return
	}

	nameSpace := pod.GetNamespace()
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		k := schedulerframework.GetNamespacedName(nameSpace, volume.PersistentVolumeClaim.ClaimName)
		if _, found := data.pvcNamespacePodMap[k]; found {
			delete(data.pvcNamespacePodMap[k], pod.GetName())
			if len(data.pvcNamespacePodMap[k]) == 0 {
				delete(data.pvcNamespacePodMap, k)
			}
		}
	}
}

func newInternalBasicSnapshotData() *internalBasicSnapshotData {
	return &internalBasicSnapshotData{
		nodeInfoMap:          make(map[string]*schedulerframework.NodeInfo),
		pvcNamespacePodMap:   make(map[string]map[string]bool),
		globalResourceClaims: make(map[dynamicresources.ResourceClaimRef]*resourceapi.ResourceClaim),
	}
}

func (data *internalBasicSnapshotData) clone() *internalBasicSnapshotData {
	clonedNodeInfoMap := make(map[string]*schedulerframework.NodeInfo)
	for k, v := range data.nodeInfoMap {
		clonedNodeInfoMap[k] = v.Snapshot()
	}
	clonedPvcNamespaceNodeMap := make(map[string]map[string]bool)
	for k, v := range data.pvcNamespacePodMap {
		clonedPvcNamespaceNodeMap[k] = make(map[string]bool)
		for k1, v1 := range v {
			clonedPvcNamespaceNodeMap[k][k1] = v1
		}
	}
	clonedGlobalResourceClaims := make(map[dynamicresources.ResourceClaimRef]*resourceapi.ResourceClaim)
	for k, v := range data.globalResourceClaims {
		clonedGlobalResourceClaims[k] = v.DeepCopy()
	}
	return &internalBasicSnapshotData{
		nodeInfoMap:          clonedNodeInfoMap,
		pvcNamespacePodMap:   clonedPvcNamespaceNodeMap,
		globalResourceClaims: clonedGlobalResourceClaims,
	}
}

func (data *internalBasicSnapshotData) addNode(node *NodeResourceInfo) error {
	if _, found := data.nodeInfoMap[node.Node.Name]; found {
		return fmt.Errorf("node %s already in snapshot", node.Node.Name)
	}
	nodeInfo := schedulerframework.NewNodeInfo()
	nodeInfo.SetNodeWithDynamicResources(node.Node, node.DynamicResources)
	data.nodeInfoMap[node.Node.Name] = nodeInfo
	return nil
}

func (data *internalBasicSnapshotData) addNodes(nodes []*NodeResourceInfo) error {
	for _, node := range nodes {
		if err := data.addNode(node); err != nil {
			return err
		}
	}
	return nil
}

func (data *internalBasicSnapshotData) removeNode(nodeName string) error {
	if _, found := data.nodeInfoMap[nodeName]; !found {
		return ErrNodeNotFound
	}
	for _, pod := range data.nodeInfoMap[nodeName].Pods {
		data.removePvcUsedByPod(pod.Pod)
	}
	delete(data.nodeInfoMap, nodeName)
	return nil
}

func (data *internalBasicSnapshotData) addPod(pod *PodResourceInfo, nodeName string) error {
	if _, found := data.nodeInfoMap[nodeName]; !found {
		return ErrNodeNotFound
	}
	podOwnedClaims, globalClaims := dynamicresources.SplitClaimsByOwnership(pod.DynamicResourceRequests.ResourceClaims)
	var claims []*resourceapi.ResourceClaim
	for _, claim := range podOwnedClaims {
		claims = append(claims, claim.DeepCopy())
	}
	// Save non-pod-owned claims in a shared location for this fork.
	for _, claim := range globalClaims {
		ref := dynamicresources.ResourceClaimRef{Name: claim.Name, Namespace: claim.Namespace}
		globalClaim, found := data.globalResourceClaims[ref]
		if !found || !dynamicresources.ClaimAllocated(globalClaim) {
			// First time we're adding this shared claim, or the first time we're adding an allocation for it.
			globalClaim = claim.DeepCopy()
			data.globalResourceClaims[ref] = globalClaim
		}
		// Swap the global claim in PodInfo to a pointer to a shared one.
		claims = append(claims, globalClaim)
		// TODO(DRA): Verify that allocations match.
	}
	data.nodeInfoMap[nodeName].AddPodWithDynamicRequests(pod.Pod, schedulerframework.PodDynamicResourceRequests{ResourceClaims: claims})
	data.addPvcUsedByPod(pod.Pod)
	return nil
}

func (data *internalBasicSnapshotData) removePod(namespace, podName, nodeName string) (*PodResourceInfo, error) {
	nodeInfo, found := data.nodeInfoMap[nodeName]
	if !found {
		return nil, ErrNodeNotFound
	}
	logger := klog.Background()
	var foundPodInfo *schedulerframework.PodInfo
	for _, podInfo := range nodeInfo.Pods {
		if podInfo.Pod.Namespace == namespace && podInfo.Pod.Name == podName {
			foundPodInfo = podInfo
			break
		}
	}
	if foundPodInfo == nil {
		return nil, fmt.Errorf("pod %s/%s not in snapshot", namespace, podName)
	}
	data.removePvcUsedByPod(foundPodInfo.Pod)
	err := nodeInfo.RemovePod(logger, foundPodInfo.Pod)
	if err != nil {
		data.addPvcUsedByPod(foundPodInfo.Pod)
		return nil, fmt.Errorf("cannot remove pod; %v", err)
	}

	var clearedClaims []*resourceapi.ResourceClaim
	podOwnedClaims, globalClaims := dynamicresources.SplitClaimsByOwnership(foundPodInfo.DynamicResourceRequests.ResourceClaims)
	for _, claim := range podOwnedClaims {
		dynamicresources.DeallocateClaimInPlace(claim)
		dynamicresources.ClearPodReservationInPlace(claim, foundPodInfo.Pod)
		clearedClaims = append(clearedClaims, claim)
	}
	for _, claim := range globalClaims {
		// Remove the pod's reservation on the shared claim. Removing the last reservation should deallocate the claim.
		// This operates on a pointer to a claim shared by pods in the snapshot.
		dynamicresources.ClearPodReservationInPlace(claim, foundPodInfo.Pod)
		// Don't return the shared pointer outside, too messy.
		clearedClaims = append(clearedClaims, claim.DeepCopy())
	}

	clearedPod := foundPodInfo.Pod.DeepCopy()
	clearedPod.Spec.NodeName = ""

	return &PodResourceInfo{Pod: clearedPod, DynamicResourceRequests: schedulerframework.PodDynamicResourceRequests{ResourceClaims: clearedClaims}}, nil
}

// NewBasicClusterSnapshot creates instances of BasicClusterSnapshot.
func NewBasicClusterSnapshot() *BasicClusterSnapshot {
	snapshot := &BasicClusterSnapshot{}
	snapshot.Clear()
	return snapshot
}

func (snapshot *BasicClusterSnapshot) getInternalData() *internalBasicSnapshotData {
	return snapshot.data[len(snapshot.data)-1]
}

// AddNode adds node to the snapshot.
func (snapshot *BasicClusterSnapshot) AddNode(node *NodeResourceInfo) error {
	return snapshot.getInternalData().addNode(node)
}

// AddNodes adds nodes in batch to the snapshot.
func (snapshot *BasicClusterSnapshot) AddNodes(nodes []*NodeResourceInfo) error {
	return snapshot.getInternalData().addNodes(nodes)
}

// AddNodeWithPods adds a node and set of pods to be scheduled to this node to the snapshot.
func (snapshot *BasicClusterSnapshot) AddNodeWithPods(node *NodeResourceInfo, pods []*PodResourceInfo) error {
	if err := snapshot.AddNode(node); err != nil {
		return err
	}
	for _, pod := range pods {
		if err := snapshot.AddPod(pod, node.Node.Name); err != nil {
			return err
		}
	}
	return nil
}

// RemoveNode removes nodes (and pods scheduled to it) from the snapshot.
func (snapshot *BasicClusterSnapshot) RemoveNode(nodeName string) error {
	return snapshot.getInternalData().removeNode(nodeName)
}

// AddPod adds pod to the snapshot and schedules it to given node.
func (snapshot *BasicClusterSnapshot) AddPod(pod *PodResourceInfo, nodeName string) error {
	return snapshot.getInternalData().addPod(pod, nodeName)
}

// RemovePod removes pod from the snapshot.
func (snapshot *BasicClusterSnapshot) RemovePod(namespace, podName, nodeName string) (*PodResourceInfo, error) {
	return snapshot.getInternalData().removePod(namespace, podName, nodeName)
}

// IsPVCUsedByPods returns if the pvc is used by any pod
func (snapshot *BasicClusterSnapshot) IsPVCUsedByPods(key string) bool {
	return snapshot.getInternalData().isPVCUsedByPods(key)
}

// Fork creates a fork of snapshot state. All modifications can later be reverted to moment of forking via Revert()
func (snapshot *BasicClusterSnapshot) Fork() {
	forkData := snapshot.getInternalData().clone()
	snapshot.data = append(snapshot.data, forkData)
}

// Revert reverts snapshot state to moment of forking.
func (snapshot *BasicClusterSnapshot) Revert() {
	if len(snapshot.data) == 1 {
		return
	}
	snapshot.data = snapshot.data[:len(snapshot.data)-1]
}

// Commit commits changes done after forking.
func (snapshot *BasicClusterSnapshot) Commit() error {
	if len(snapshot.data) <= 1 {
		// do nothing
		return nil
	}
	snapshot.data = append(snapshot.data[:len(snapshot.data)-2], snapshot.data[len(snapshot.data)-1])
	return nil
}

// Clear reset cluster snapshot to empty, unforked state
func (snapshot *BasicClusterSnapshot) Clear() {
	baseData := newInternalBasicSnapshotData()
	snapshot.data = []*internalBasicSnapshotData{baseData}
	snapshot.globalResourceSlices = []*resourceapi.ResourceSlice{}
	snapshot.allResourceClaims = map[dynamicresources.ResourceClaimRef]*resourceapi.ResourceClaim{}
	snapshot.resourceClaimAllocations = map[types.UID]*resourceapi.ResourceClaim{}
	snapshot.allDeviceClasses = map[string]*resourceapi.DeviceClass{}
}

// implementation of SharedLister interface

type basicClusterSnapshotNodeLister BasicClusterSnapshot
type basicClusterSnapshotStorageLister BasicClusterSnapshot

// NodeInfos exposes snapshot as NodeInfoLister.
func (snapshot *BasicClusterSnapshot) NodeInfos() schedulerframework.NodeInfoLister {
	return (*basicClusterSnapshotNodeLister)(snapshot)
}

// StorageInfos exposes snapshot as StorageInfoLister.
func (snapshot *BasicClusterSnapshot) StorageInfos() schedulerframework.StorageInfoLister {
	return (*basicClusterSnapshotStorageLister)(snapshot)
}

func (snapshot *BasicClusterSnapshot) ResourceClaims() schedulerframework.ResourceClaimTracker {
	return (*basicClusterSnapshotResourceClaimsTracker)(snapshot)
}

func (snapshot *BasicClusterSnapshot) ResourceSlices() schedulerframework.ResourceSliceLister {
	return (*basicClusterSnapshotResourceSliceLister)(snapshot)
}

func (snapshot *BasicClusterSnapshot) DeviceClasses() schedulerframework.DeviceClassLister {
	return (*basicClusterSnapshotDeviceClassLister)(snapshot)
}

// List returns the list of nodes in the snapshot.
func (snapshot *basicClusterSnapshotNodeLister) List() ([]*schedulerframework.NodeInfo, error) {
	return (*BasicClusterSnapshot)(snapshot).getInternalData().listNodeInfos()
}

// HavePodsWithAffinityList returns the list of nodes with at least one pods with inter-pod affinity
func (snapshot *basicClusterSnapshotNodeLister) HavePodsWithAffinityList() ([]*schedulerframework.NodeInfo, error) {
	return (*BasicClusterSnapshot)(snapshot).getInternalData().listNodeInfosThatHavePodsWithAffinityList()
}

// HavePodsWithRequiredAntiAffinityList returns the list of NodeInfos of nodes with pods with required anti-affinity terms.
func (snapshot *basicClusterSnapshotNodeLister) HavePodsWithRequiredAntiAffinityList() ([]*schedulerframework.NodeInfo, error) {
	return (*BasicClusterSnapshot)(snapshot).getInternalData().listNodeInfosThatHavePodsWithRequiredAntiAffinityList()
}

// Returns the NodeInfo of the given node name.
func (snapshot *basicClusterSnapshotNodeLister) Get(nodeName string) (*schedulerframework.NodeInfo, error) {
	return (*BasicClusterSnapshot)(snapshot).getInternalData().getNodeInfo(nodeName)
}

// Returns the IsPVCUsedByPods in a given key.
func (snapshot *basicClusterSnapshotStorageLister) IsPVCUsedByPods(key string) bool {
	return (*BasicClusterSnapshot)(snapshot).getInternalData().isPVCUsedByPods(key)
}

type basicClusterSnapshotResourceClaimsTracker BasicClusterSnapshot

func (snapshot *basicClusterSnapshotResourceClaimsTracker) GetOriginal(namespace, claimName string) (*resourceapi.ResourceClaim, error) {
	// TODO implement me
	panic("implement me")
}

func (snapshot *basicClusterSnapshotResourceClaimsTracker) RemoveClaimPendingAllocation(claimUid types.UID) (found bool) {
	// TODO implement me
	panic("implement me")
}

func (snapshot *basicClusterSnapshotResourceClaimsTracker) AssumeClaimAfterApiCall(claim *resourceapi.ResourceClaim) error {
	// TODO implement me
	panic("implement me")
}

func (snapshot *basicClusterSnapshotResourceClaimsTracker) AssumedClaimRestore(namespace, claimName string) {
	// TODO implement me
	panic("implement me")
}

func (snapshot *basicClusterSnapshotResourceClaimsTracker) scheduledResourceClaimsIterate(iterFn func(*resourceapi.ResourceClaim) bool) error {
	data := (*BasicClusterSnapshot)(snapshot).getInternalData()
	nodeInfos, err := data.listNodeInfos()
	if err != nil {
		return err
	}
	// Iterate over pod-owned claims for scheduled pods.
	for _, nodeInfo := range nodeInfos {
		for _, podInfo := range nodeInfo.Pods {
			for _, claim := range podInfo.DynamicResourceRequests.ResourceClaims {
				if cont := iterFn(claim); !cont {
					return nil
				}
			}
		}
	}
	// Iterate over global claims used by scheduled pods.
	for _, claim := range data.globalResourceClaims {
		if cont := iterFn(claim); !cont {
			return nil
		}
	}
	return nil
}

func (snapshot *basicClusterSnapshotResourceClaimsTracker) Get(namespace, claimName string) (*resourceapi.ResourceClaim, error) {
	// First check if we're tracking the claim in the snapshot. If so, we return it along with possible allocations etc.
	var result *resourceapi.ResourceClaim
	err := snapshot.scheduledResourceClaimsIterate(func(claim *resourceapi.ResourceClaim) bool {
		if claim.Namespace == namespace && claim.Name == claimName {
			result = claim
			return false
		}
		return true
	})
	if result != nil {
		return result, nil
	}
	// This should mean that the request is for a claim for a pod that isn't scheduled - fall back to querying the original objects.
	if claim, found := snapshot.allResourceClaims[dynamicresources.ResourceClaimRef{Namespace: namespace, Name: claimName}]; found {
		return claim, err
	}
	return nil, fmt.Errorf("claim %s/%s not found", namespace, claimName)
}

func (snapshot *basicClusterSnapshotResourceClaimsTracker) List() ([]*resourceapi.ResourceClaim, error) {
	var result []*resourceapi.ResourceClaim
	trackedClaims := map[dynamicresources.ResourceClaimRef]bool{}
	err := snapshot.scheduledResourceClaimsIterate(func(claim *resourceapi.ResourceClaim) bool {
		result = append(result, claim)
		trackedClaims[dynamicresources.ResourceClaimRef{Name: claim.Name, Namespace: claim.Namespace}] = true
		return true
	})
	for _, claim := range snapshot.allResourceClaims {
		if !trackedClaims[dynamicresources.ResourceClaimRef{Name: claim.Name, Namespace: claim.Namespace}] {
			result = append(result, claim)
		}
	}
	return result, err
}

func (snapshot *basicClusterSnapshotResourceClaimsTracker) ListAllAllocated() ([]*resourceapi.ResourceClaim, error) {
	claims, err := snapshot.List()
	if err != nil {
		return nil, err
	}
	var result []*resourceapi.ResourceClaim
	for _, claim := range claims {
		if dynamicresources.ClaimAllocated(claim) {
			result = append(result, claim)
		}
	}
	return result, nil
}

func (snapshot *basicClusterSnapshotResourceClaimsTracker) ClaimHasPendingAllocation(claimUid types.UID) bool {
	return false
}

func (snapshot *basicClusterSnapshotResourceClaimsTracker) SignalClaimPendingAllocation(claimUid types.UID, allocatedClaim *resourceapi.ResourceClaim) {
	snapshot.resourceClaimAllocations[claimUid] = allocatedClaim
}

type basicClusterSnapshotResourceSliceLister BasicClusterSnapshot

func (snapshot *basicClusterSnapshotResourceSliceLister) List() ([]*resourceapi.ResourceSlice, error) {
	var result []*resourceapi.ResourceSlice
	nodeInfos, err := (*BasicClusterSnapshot)(snapshot).getInternalData().listNodeInfos()
	if err != nil {
		return nil, err
	}
	for _, nodeInfo := range nodeInfos {
		result = append(result, nodeInfo.DynamicResources().ResourceSlices...)
	}
	result = append(result, snapshot.globalResourceSlices...)
	return result, nil
}

type basicClusterSnapshotDeviceClassLister BasicClusterSnapshot

func (snapshot *basicClusterSnapshotDeviceClassLister) Get(className string) (*resourceapi.DeviceClass, error) {
	class, found := snapshot.allDeviceClasses[className]
	if !found {
		return nil, fmt.Errorf("DeviceClass %q not found", className)
	}
	return class, nil
}

func (snapshot *basicClusterSnapshotDeviceClassLister) List() ([]*resourceapi.DeviceClass, error) {
	var result []*resourceapi.DeviceClass
	for _, class := range snapshot.allDeviceClasses {
		result = append(result, class)
	}
	return result, nil
}
