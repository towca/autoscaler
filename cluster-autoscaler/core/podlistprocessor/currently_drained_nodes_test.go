/*
Copyright 2022 The Kubernetes Authors.

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

package podlistprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/context"
	"k8s.io/autoscaler/cluster-autoscaler/core/scaledown"
	"k8s.io/autoscaler/cluster-autoscaler/core/scaledown/status"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/clustersnapshot"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	. "k8s.io/autoscaler/cluster-autoscaler/utils/test"
)

func TestCurrentlyDrainedNodesPodListProcessor(t *testing.T) {
	testCases := []struct {
		name         string
		drainedNodes []string
		nodes        []*clustersnapshot.NodeResourceInfo
		pods         []*clustersnapshot.PodResourceInfo

		unschedulablePods []*clustersnapshot.PodResourceInfo
		wantPods          []*clustersnapshot.PodResourceInfo
	}{
		{
			name: "no nodes, no unschedulable pods",
		},
		{
			name: "no nodes, some unschedulable pods",
			unschedulablePods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
				{Pod: BuildTestPod("p2", 200, 1)},
			},
			wantPods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
				{Pod: BuildTestPod("p2", 200, 1)},
			},
		},
		{
			name:         "single node undergoing deletion",
			drainedNodes: []string{"n"},
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n", 1000, 10)},
			},
			pods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildScheduledTestPod("p1", 100, 1, "n")},
				{Pod: BuildScheduledTestPod("p2", 200, 1, "n")},
			},
			wantPods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
				{Pod: BuildTestPod("p2", 200, 1)},
			},
		},
		{
			name:         "single node undergoing deletion, pods with deletion timestamp set",
			drainedNodes: []string{"n"},
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n", 1000, 10)},
			},
			pods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildScheduledTestPod("p1", 100, 1, "n")},
				{Pod: BuildTestPod("p2", 200, 1, WithNodeName("n"), WithDeletionTimestamp(time.Now()))},
			},
			wantPods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
			},
		},
		{
			name:         "single empty node undergoing deletion",
			drainedNodes: []string{"n"},
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n", 1000, 10)},
			},
		},
		{
			name:         "single node undergoing deletion, unschedulable pods",
			drainedNodes: []string{"n"},
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n", 1000, 10)},
			},
			pods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildScheduledTestPod("p1", 100, 1, "n")},
				{Pod: BuildScheduledTestPod("p2", 200, 1, "n")},
			},
			unschedulablePods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p3", 300, 1)},
				{Pod: BuildTestPod("p4", 400, 1)},
			},
			wantPods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
				{Pod: BuildTestPod("p2", 200, 1)},
				{Pod: BuildTestPod("p3", 300, 1)},
				{Pod: BuildTestPod("p4", 400, 1)},
			},
		},
		{
			name: "single ready node",
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n", 1000, 10)},
			},
			pods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildScheduledTestPod("p1", 100, 1, "n")},
				{Pod: BuildScheduledTestPod("p2", 200, 1, "n")},
			},
		},
		{
			name: "single ready node, unschedulable pods",
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n", 1000, 10)},
			},
			pods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildScheduledTestPod("p1", 100, 1, "n")},
				{Pod: BuildScheduledTestPod("p2", 200, 1, "n")},
			},
			unschedulablePods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p3", 300, 1)},
				{Pod: BuildTestPod("p4", 400, 1)},
			},
			wantPods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p3", 300, 1)},
				{Pod: BuildTestPod("p4", 400, 1)},
			},
		},
		{
			name:         "multiple nodes, all undergoing deletion",
			drainedNodes: []string{"n1", "n2"},
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n1", 1000, 10)},
				{Node: BuildTestNode("n2", 1000, 10)},
			},
			pods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildScheduledTestPod("p1", 100, 1, "n1")},
				{Pod: BuildScheduledTestPod("p2", 200, 1, "n1")},
				{Pod: BuildScheduledTestPod("p3", 300, 1, "n2")},
				{Pod: BuildScheduledTestPod("p4", 400, 1, "n2")},
			},
			wantPods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
				{Pod: BuildTestPod("p2", 200, 1)},
				{Pod: BuildTestPod("p3", 300, 1)},
				{Pod: BuildTestPod("p4", 400, 1)},
			},
		},
		{
			name:         "multiple nodes, some undergoing deletion",
			drainedNodes: []string{"n1"},
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n1", 1000, 10)},
				{Node: BuildTestNode("n2", 1000, 10)},
			},
			pods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildScheduledTestPod("p1", 100, 1, "n1")},
				{Pod: BuildScheduledTestPod("p2", 200, 1, "n1")},
				{Pod: BuildScheduledTestPod("p3", 300, 1, "n2")},
				{Pod: BuildScheduledTestPod("p4", 400, 1, "n2")},
			},
			wantPods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
				{Pod: BuildTestPod("p2", 200, 1)},
			},
		},
		{
			name: "multiple nodes, no undergoing deletion",
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n1", 1000, 10)},
				{Node: BuildTestNode("n2", 1000, 10)},
			},
			pods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildScheduledTestPod("p1", 100, 1, "n1")},
				{Pod: BuildScheduledTestPod("p2", 200, 1, "n1")},
				{Pod: BuildScheduledTestPod("p3", 300, 1, "n2")},
				{Pod: BuildScheduledTestPod("p4", 400, 1, "n2")},
			},
		},
		{
			name:         "single node, non-recreatable pods filtered out",
			drainedNodes: []string{"n"},
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n", 1000, 10)},
			},
			pods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildScheduledTestPod("p1", 100, 1, "n")},
				{Pod: SetRSPodSpec(BuildScheduledTestPod("p2", 200, 1, "n"), "rs")},
				{Pod: SetDSPodSpec(BuildScheduledTestPod("p3", 300, 1, "n"))},
				{Pod: SetMirrorPodSpec(BuildScheduledTestPod("p4", 400, 1, "n"))},
				{Pod: SetStaticPodSpec(BuildScheduledTestPod("p5", 500, 1, "n"))},
			},
			wantPods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
				{Pod: SetRSPodSpec(BuildTestPod("p2", 200, 1), "rs")},
			},
		},
		{
			name: "unschedulable pods, non-recreatable pods not filtered out",
			unschedulablePods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
				{Pod: SetRSPodSpec(BuildTestPod("p2", 200, 1), "rs")},
				{Pod: SetDSPodSpec(BuildTestPod("p3", 300, 1))},
				{Pod: SetMirrorPodSpec(BuildTestPod("p4", 400, 1))},
				{Pod: SetStaticPodSpec(BuildTestPod("p5", 500, 1))},
			},
			wantPods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
				{Pod: SetRSPodSpec(BuildTestPod("p2", 200, 1), "rs")},
				{Pod: SetDSPodSpec(BuildTestPod("p3", 300, 1))},
				{Pod: SetMirrorPodSpec(BuildTestPod("p4", 400, 1))},
				{Pod: SetStaticPodSpec(BuildTestPod("p5", 500, 1))},
			},
		},
		{
			name:         "everything works together",
			drainedNodes: []string{"n1", "n3", "n5"},
			nodes: []*clustersnapshot.NodeResourceInfo{
				{Node: BuildTestNode("n1", 1000, 10)},
				{Node: BuildTestNode("n2", 1000, 10)},
				{Node: BuildTestNode("n3", 1000, 10)},
				{Node: BuildTestNode("n4", 1000, 10)},
				{Node: BuildTestNode("n5", 1000, 10)},
			},
			pods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildScheduledTestPod("p1", 100, 1, "n1")},
				{Pod: BuildScheduledTestPod("p2", 200, 1, "n1")},
				{Pod: SetRSPodSpec(BuildScheduledTestPod("p3", 300, 1, "n1"), "rs")},
				{Pod: SetDSPodSpec(BuildScheduledTestPod("p4", 400, 1, "n1"))},
				{Pod: BuildScheduledTestPod("p5", 500, 1, "n2")},
				{Pod: BuildScheduledTestPod("p6", 600, 1, "n2")},
				{Pod: BuildScheduledTestPod("p7", 700, 1, "n3")},
				{Pod: SetStaticPodSpec(BuildScheduledTestPod("p8", 800, 1, "n3"))},
				{Pod: SetMirrorPodSpec(BuildScheduledTestPod("p9", 900, 1, "n3"))},
			},
			unschedulablePods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p10", 1000, 1)},
				{Pod: SetMirrorPodSpec(BuildTestPod("p11", 1100, 1))},
				{Pod: SetStaticPodSpec(BuildTestPod("p12", 1200, 1))},
			},
			wantPods: []*clustersnapshot.PodResourceInfo{
				{Pod: BuildTestPod("p1", 100, 1)},
				{Pod: BuildTestPod("p2", 200, 1)},
				{Pod: SetRSPodSpec(BuildTestPod("p3", 300, 1), "rs")},
				{Pod: BuildTestPod("p7", 700, 1)},
				{Pod: BuildTestPod("p10", 1000, 1)},
				{Pod: SetMirrorPodSpec(BuildTestPod("p11", 1100, 1))},
				{Pod: SetStaticPodSpec(BuildTestPod("p12", 1200, 1))},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.AutoscalingContext{
				ScaleDownActuator: &mockActuator{&mockActuationStatus{tc.drainedNodes}},
				ClusterSnapshot:   &clustersnapshot.Handle{ClusterSnapshot: clustersnapshot.NewBasicClusterSnapshot()},
			}
			clustersnapshot.InitializeClusterSnapshotWithDynamicResourcesOrDie(t, ctx.ClusterSnapshot, tc.nodes, tc.pods)

			processor := NewCurrentlyDrainedNodesPodListProcessor()
			pods, err := processor.Process(&ctx, tc.unschedulablePods)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tc.wantPods, pods)
		})
	}
}

type mockActuator struct {
	status *mockActuationStatus
}

func (m *mockActuator) StartDeletion(_, _ []*apiv1.Node) (status.ScaleDownResult, []*status.ScaleDownNode, errors.AutoscalerError) {
	return status.ScaleDownError, []*status.ScaleDownNode{}, nil
}

func (m *mockActuator) CheckStatus() scaledown.ActuationStatus {
	return m.status
}

func (m *mockActuator) ClearResultsNotNewerThan(time.Time) {

}

func (m *mockActuator) DeletionResults() (map[string]status.NodeDeleteResult, time.Time) {
	return map[string]status.NodeDeleteResult{}, time.Now()
}

type mockActuationStatus struct {
	drainedNodes []string
}

func (m *mockActuationStatus) RecentEvictions() []*apiv1.Pod {
	return nil
}

func (m *mockActuationStatus) DeletionsInProgress() ([]string, []string) {
	return nil, m.drainedNodes
}

func (m *mockActuationStatus) DeletionResults() (map[string]status.NodeDeleteResult, time.Time) {
	return nil, time.Time{}
}

func (m *mockActuationStatus) DeletionsCount(_ string) int {
	return 0
}
