/*
Copyright 2016 The Kubernetes Authors.

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

package predicatechecker

import (
	"context"
	"fmt"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/clustersnapshot"

	"k8s.io/client-go/informers"
	v1listers "k8s.io/client-go/listers/core/v1"
	klog "k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/apis/config"
	scheduler_config "k8s.io/kubernetes/pkg/scheduler/apis/config/latest"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
	scheduler_plugins "k8s.io/kubernetes/pkg/scheduler/framework/plugins"
	schedulerframeworkruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
)

// SchedulerBasedPredicateChecker checks whether all required predicates pass for given Pod and Node.
// The verification is done by calling out to scheduler code.
type SchedulerBasedPredicateChecker struct {
	framework              schedulerframework.Framework
	delegatingSharedLister *DelegatingSchedulerSharedLister
	nodeLister             v1listers.NodeLister
	podLister              v1listers.PodLister
	lastIndex              int
}

// NewSchedulerBasedPredicateChecker builds scheduler based PredicateChecker.
func NewSchedulerBasedPredicateChecker(informerFactory informers.SharedInformerFactory, schedConfig *config.KubeSchedulerConfiguration) (*SchedulerBasedPredicateChecker, error) {
	if schedConfig == nil {
		var err error
		schedConfig, err = scheduler_config.Default()
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler config: %v", err)
		}
	}

	if len(schedConfig.Profiles) != 1 {
		return nil, fmt.Errorf("unexpected scheduler config: expected one scheduler profile only (found %d profiles)", len(schedConfig.Profiles))
	}
	sharedLister := NewDelegatingSchedulerSharedLister()

	framework, err := schedulerframeworkruntime.NewFramework(
		context.TODO(),
		scheduler_plugins.NewInTreeRegistry(),
		&schedConfig.Profiles[0],
		schedulerframeworkruntime.WithInformerFactory(informerFactory),
		schedulerframeworkruntime.WithSnapshotSharedLister(sharedLister),
		schedulerframeworkruntime.WithSharedDraManager(sharedLister),
	)

	if err != nil {
		return nil, fmt.Errorf("couldn't create scheduler framework; %v", err)
	}

	checker := &SchedulerBasedPredicateChecker{
		framework:              framework,
		delegatingSharedLister: sharedLister,
	}

	return checker, nil
}

// FitsAnyNode checks if the given pod can be placed on any of the given nodes.
func (p *SchedulerBasedPredicateChecker) FitsAnyNode(clusterSnapshot clustersnapshot.ClusterSnapshot, pod *clustersnapshot.PodResourceInfo) (string, *clustersnapshot.PodResourceInfo, error) {
	return p.FitsAnyNodeMatching(clusterSnapshot, pod, func(*schedulerframework.NodeInfo) bool {
		return true
	})
}

// FitsAnyNodeMatching checks if the given pod can be placed on any of the given nodes matching the provided function.
func (p *SchedulerBasedPredicateChecker) FitsAnyNodeMatching(clusterSnapshot clustersnapshot.ClusterSnapshot, pod *clustersnapshot.PodResourceInfo, nodeMatches func(*schedulerframework.NodeInfo) bool) (string, *clustersnapshot.PodResourceInfo, error) {
	if clusterSnapshot == nil {
		return "", nil, fmt.Errorf("ClusterSnapshot not provided")
	}

	nodeInfosList, err := clusterSnapshot.NodeInfos().List()
	if err != nil {
		// This should never happen.
		//
		// Scheduler requires interface returning error, but no implementation
		// of ClusterSnapshot ever does it.
		klog.Errorf("Error obtaining nodeInfos from schedulerLister")
		return "", nil, fmt.Errorf("error obtaining nodeInfos from schedulerLister")
	}

	p.delegatingSharedLister.UpdateDelegate(clusterSnapshot)
	defer p.delegatingSharedLister.ResetDelegate()

	state := schedulerframework.NewCycleState()
	preFilterResult, preFilterStatus, _ := p.framework.RunPreFilterPlugins(context.TODO(), state, pod.Pod)
	if !preFilterStatus.IsSuccess() {
		return "", nil, fmt.Errorf("error running pre filter plugins for pod %s; %s", pod.Name, preFilterStatus.Message())
	}

	for i := range nodeInfosList {
		nodeInfo := nodeInfosList[(p.lastIndex+i)%len(nodeInfosList)]
		if !nodeMatches(nodeInfo) {
			continue
		}

		if !preFilterResult.AllNodes() && !preFilterResult.NodeNames.Has(nodeInfo.Node().Name) {
			continue
		}

		// Be sure that the node is schedulable.
		if nodeInfo.Node().Spec.Unschedulable {
			continue
		}

		filterStatus := p.framework.RunFilterPlugins(context.TODO(), state, pod.Pod, nodeInfo)
		if !filterStatus.IsSuccess() {
			continue
		}

		reservedPod, err := p.getPodAllocations(clusterSnapshot, pod, nodeInfo.Node().Name, state)
		if err != nil {
			return "", nil, err
		}

		p.lastIndex = (p.lastIndex + i + 1) % len(nodeInfosList)
		return nodeInfo.Node().Name, reservedPod, nil
	}
	return "", nil, fmt.Errorf("cannot put pod %s on any node", pod.Name)
}

// CheckPredicates checks if the given pod can be placed on the given node.
func (p *SchedulerBasedPredicateChecker) CheckPredicates(clusterSnapshot clustersnapshot.ClusterSnapshot, pod *clustersnapshot.PodResourceInfo, nodeName string) (*PredicateError, *clustersnapshot.PodResourceInfo) {
	if clusterSnapshot == nil {
		return NewPredicateError(InternalPredicateError, "", "ClusterSnapshot not provided", nil, emptyString), nil
	}
	nodeInfo, err := clusterSnapshot.NodeInfos().Get(nodeName)
	if err != nil {
		errorMessage := fmt.Sprintf("Error obtaining NodeInfo for name %s; %v", nodeName, err)
		return NewPredicateError(InternalPredicateError, "", errorMessage, nil, emptyString), nil
	}

	p.delegatingSharedLister.UpdateDelegate(clusterSnapshot)
	defer p.delegatingSharedLister.ResetDelegate()

	state := schedulerframework.NewCycleState()
	_, preFilterStatus, _ := p.framework.RunPreFilterPlugins(context.TODO(), state, pod.Pod)
	if !preFilterStatus.IsSuccess() {
		return NewPredicateError(
			InternalPredicateError,
			"",
			preFilterStatus.Message(),
			preFilterStatus.Reasons(),
			emptyString), nil
	}

	filterStatus := p.framework.RunFilterPlugins(context.TODO(), state, pod.Pod, nodeInfo)

	if !filterStatus.IsSuccess() {
		filterName := filterStatus.Plugin()
		filterMessage := filterStatus.Message()
		filterReasons := filterStatus.Reasons()
		if filterStatus.IsRejected() {
			return NewPredicateError(
				NotSchedulablePredicateError,
				filterName,
				filterMessage,
				filterReasons,
				p.buildDebugInfo(filterName, nodeInfo)), nil
		}
		return NewPredicateError(
			InternalPredicateError,
			filterName,
			filterMessage,
			filterReasons,
			p.buildDebugInfo(filterName, nodeInfo)), nil
	}

	reservedPod, err := p.getPodAllocations(clusterSnapshot, pod, nodeName, state)
	if err != nil {
		return NewPredicateError(InternalPredicateError, "reserve", err.Error(), nil, p.buildDebugInfo("reserve", nodeInfo)), nil
	}

	return nil, reservedPod
}

func (p *SchedulerBasedPredicateChecker) getPodAllocations(snapshot clustersnapshot.ClusterSnapshot, pod *clustersnapshot.PodResourceInfo, nodeName string, stateAfterFilters *schedulerframework.CycleState) (*clustersnapshot.PodResourceInfo, error) {
	// Clean the state on exit.
	defer snapshot.ClearResourceClaimAllocations()
	// Clean the state now, just to be safe in case something else didn't clean up.
	snapshot.ClearResourceClaimAllocations()

	reserveStatus := p.framework.RunReservePluginsReserve(context.TODO(), stateAfterFilters, pod.Pod, nodeName)
	if !reserveStatus.IsSuccess() {
		return nil, fmt.Errorf(reserveStatus.Message())
	}
	allocationsNeededForPod := snapshot.GetResourceClaimAllocations()
	reservedPod, err := pod.AllocateClaims(allocationsNeededForPod)
	if err != nil {
		return nil, err
	}
	return reservedPod, nil
}

func (p *SchedulerBasedPredicateChecker) buildDebugInfo(filterName string, nodeInfo *schedulerframework.NodeInfo) func() string {
	switch filterName {
	case "TaintToleration":
		taints := nodeInfo.Node().Spec.Taints
		return func() string {
			return fmt.Sprintf("taints on node: %#v", taints)
		}
	default:
		return emptyString
	}
}
