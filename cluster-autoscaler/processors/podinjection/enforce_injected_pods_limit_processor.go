/*
Copyright 2024 The Kubernetes Authors.

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

package podinjection

import (
	"k8s.io/autoscaler/cluster-autoscaler/context"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/clustersnapshot"
)

// EnforceInjectedPodsLimitProcessor is a PodListProcessor used to limit the number of injected fake pods.
type EnforceInjectedPodsLimitProcessor struct {
	podLimit int
}

// NewEnforceInjectedPodsLimitProcessor return an instance of EnforceInjectedPodsLimitProcessor
func NewEnforceInjectedPodsLimitProcessor(podLimit int) *EnforceInjectedPodsLimitProcessor {
	return &EnforceInjectedPodsLimitProcessor{
		podLimit: podLimit,
	}
}

// Process filters unschedulablePods and enforces the limit of the number of injected pods
func (p *EnforceInjectedPodsLimitProcessor) Process(ctx *context.AutoscalingContext, unschedulablePods []*clustersnapshot.PodResourceInfo) ([]*clustersnapshot.PodResourceInfo, error) {

	numberOfFakePodsToRemove := len(unschedulablePods) - p.podLimit
	var unschedulablePodsAfterProcessing []*clustersnapshot.PodResourceInfo

	for _, pod := range unschedulablePods {
		if IsFake(pod.Pod) && numberOfFakePodsToRemove > 0 {
			numberOfFakePodsToRemove -= 1
			continue
		}

		unschedulablePodsAfterProcessing = append(unschedulablePodsAfterProcessing, pod)
	}

	return unschedulablePodsAfterProcessing, nil
}

// CleanUp is called at CA termination
func (p *EnforceInjectedPodsLimitProcessor) CleanUp() {
}
