/*
Copyright 2018 The Kubernetes Authors.

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

package pods

import (
	"k8s.io/autoscaler/cluster-autoscaler/context"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/clustersnapshot"
)

// PodListProcessor processes lists of unschedulable pods.
type PodListProcessor interface {
	Process(
		context *context.AutoscalingContext,
		unschedulablePods []*clustersnapshot.PodResourceInfo) ([]*clustersnapshot.PodResourceInfo, error)
	CleanUp()
}

// NoOpPodListProcessor is returning pod lists without processing them.
type NoOpPodListProcessor struct {
}

// NewDefaultPodListProcessor creates an instance of PodListProcessor.
func NewDefaultPodListProcessor() PodListProcessor {
	return &NoOpPodListProcessor{}
}

// Process processes lists of unschedulable and scheduled pods before scaling of the cluster.
func (p *NoOpPodListProcessor) Process(
	context *context.AutoscalingContext,
	unschedulablePods []*clustersnapshot.PodResourceInfo) ([]*clustersnapshot.PodResourceInfo, error) {
	return unschedulablePods, nil
}

// CleanUp cleans up the processor's internal structures.
func (p *NoOpPodListProcessor) CleanUp() {
}

// CombinedPodListProcessor is a list of PodListProcessors
type CombinedPodListProcessor struct {
	processors []PodListProcessor
}

// NewCombinedPodListProcessor construct CombinedPodListProcessor.
func NewCombinedPodListProcessor(processors []PodListProcessor) *CombinedPodListProcessor {
	return &CombinedPodListProcessor{processors}
}

// AddProcessor append processor to the list.
func (p *CombinedPodListProcessor) AddProcessor(processor PodListProcessor) {
	p.processors = append(p.processors, processor)
}

// Process runs sub-processors sequentially
func (p *CombinedPodListProcessor) Process(ctx *context.AutoscalingContext, unschedulablePods []*clustersnapshot.PodResourceInfo) ([]*clustersnapshot.PodResourceInfo, error) {
	var err error
	for _, processor := range p.processors {
		unschedulablePods, err = processor.Process(ctx, unschedulablePods)
		if err != nil {
			return nil, err
		}
	}
	return unschedulablePods, nil
}

// CleanUp cleans up the processor's internal structures.
func (p *CombinedPodListProcessor) CleanUp() {
	for _, processor := range p.processors {
		processor.CleanUp()
	}
}
