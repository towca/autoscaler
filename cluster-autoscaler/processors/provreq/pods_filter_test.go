/*
Copyright 2023 The Kubernetes Authors.

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

package provreq

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/autoscaler/cluster-autoscaler/apis/provisioningrequest/autoscaling.x-k8s.io/v1beta1"
	"k8s.io/autoscaler/cluster-autoscaler/context"
	"k8s.io/autoscaler/cluster-autoscaler/provisioningrequest/pods"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/clustersnapshot"
	. "k8s.io/autoscaler/cluster-autoscaler/utils/test"
	"k8s.io/client-go/tools/record"
)

func TestProvisioningRequestPodsFilter(t *testing.T) {
	prPod1 := &clustersnapshot.PodResourceInfo{Pod: BuildTestPod("pr-pod-1", 500, 10)}
	prPod1.Pod.Annotations[v1beta1.ProvisioningRequestPodAnnotationKey] = "pr-class"

	prPod2 := &clustersnapshot.PodResourceInfo{Pod: BuildTestPod("pr-pod-2", 500, 10)}
	prPod2.Pod.Annotations[pods.DeprecatedProvisioningRequestPodAnnotationKey] = "pr-class-2"

	pod1 := &clustersnapshot.PodResourceInfo{Pod: BuildTestPod("pod-1", 500, 10)}
	pod2 := &clustersnapshot.PodResourceInfo{Pod: BuildTestPod("pod-2", 500, 10)}

	testCases := map[string]struct {
		unschedulableCandidates []*clustersnapshot.PodResourceInfo
		expectedUnscheduledPods []*clustersnapshot.PodResourceInfo
	}{
		"ProvisioningRequest consumer is filtered out": {
			unschedulableCandidates: []*clustersnapshot.PodResourceInfo{prPod1, pod1},
			expectedUnscheduledPods: []*clustersnapshot.PodResourceInfo{pod1},
		},
		"Different ProvisioningRequest consumers are filtered out": {
			unschedulableCandidates: []*clustersnapshot.PodResourceInfo{prPod1, prPod2, pod1},
			expectedUnscheduledPods: []*clustersnapshot.PodResourceInfo{pod1},
		},
		"No pod is filtered": {
			unschedulableCandidates: []*clustersnapshot.PodResourceInfo{pod1, pod2},
			expectedUnscheduledPods: []*clustersnapshot.PodResourceInfo{pod1, pod2},
		},
		"Empty unschedulable pods list": {
			unschedulableCandidates: []*clustersnapshot.PodResourceInfo{},
			expectedUnscheduledPods: []*clustersnapshot.PodResourceInfo{},
		},
		"All ProvisioningRequest consumers are filtered out": {
			unschedulableCandidates: []*clustersnapshot.PodResourceInfo{prPod1, prPod2},
			expectedUnscheduledPods: []*clustersnapshot.PodResourceInfo{},
		},
	}
	for _, test := range testCases {
		eventRecorder := record.NewFakeRecorder(10)
		ctx := &context.AutoscalingContext{AutoscalingKubeClients: context.AutoscalingKubeClients{Recorder: eventRecorder}}
		filter := NewProvisioningRequestPodsFilter(NewDefautlEventManager())
		got, _ := filter.Process(ctx, test.unschedulableCandidates)
		assert.ElementsMatch(t, got, test.expectedUnscheduledPods)
		if len(test.expectedUnscheduledPods) < len(test.expectedUnscheduledPods) {
			select {
			case event := <-eventRecorder.Events:
				assert.Contains(t, event, "Unschedulable pod didn't trigger scale-up, because it's consuming ProvisioningRequest default/pr-class")
			case <-time.After(1 * time.Second):
				t.Errorf("Timeout waiting for event")
			}
		}
	}
}

func TestEventManager(t *testing.T) {
	eventLimit := 5
	eventManager := &defaultEventManager{limit: eventLimit}
	prFilter := NewProvisioningRequestPodsFilter(eventManager)
	eventRecorder := record.NewFakeRecorder(10)
	ctx := &context.AutoscalingContext{AutoscalingKubeClients: context.AutoscalingKubeClients{Recorder: eventRecorder}}
	unscheduledPods := []*clustersnapshot.PodResourceInfo{{Pod: BuildTestPod("pod", 500, 10)}}

	for i := 0; i < 10; i++ {
		prPod := &clustersnapshot.PodResourceInfo{Pod: BuildTestPod(fmt.Sprintf("pr-pod-%d", i), 10, 10)}
		prPod.Pod.Annotations[v1beta1.ProvisioningRequestPodAnnotationKey] = "pr-class"
		unscheduledPods = append(unscheduledPods, prPod)
	}
	got, err := prFilter.Process(ctx, unscheduledPods)
	assert.NoError(t, err)
	if len(got) != 1 {
		t.Errorf("Want 1 unschedulable pod, got: %v", got)
	}
	assert.Equal(t, eventManager.loggedEvents, eventLimit)
	for i := 0; i < eventLimit; i++ {
		select {
		case event := <-eventRecorder.Events:
			assert.Contains(t, event, "Unschedulable pod didn't trigger scale-up, because it's consuming ProvisioningRequest default/pr-class")
		case <-time.After(1 * time.Second):
			t.Errorf("Timeout waiting for event")
		}
	}
	select {
	case <-eventRecorder.Events:
		t.Errorf("Receive event after reaching event limit")
	case <-time.After(1 * time.Millisecond):
		return
	}
}
