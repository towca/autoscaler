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

package predicatechecker

import (
	"k8s.io/autoscaler/cluster-autoscaler/simulator/clustersnapshot"

	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

// PredicateChecker checks whether all required predicates pass for given Pod and Node.
type PredicateChecker interface {
	FitsAnyNode(clusterSnapshot clustersnapshot.ClusterSnapshot, pod *clustersnapshot.PodResourceInfo) (string, *clustersnapshot.PodResourceInfo, error)
	FitsAnyNodeMatching(clusterSnapshot clustersnapshot.ClusterSnapshot, pod *clustersnapshot.PodResourceInfo, nodeMatches func(*schedulerframework.NodeInfo) bool) (string, *clustersnapshot.PodResourceInfo, error)
	CheckPredicates(clusterSnapshot clustersnapshot.ClusterSnapshot, pod *clustersnapshot.PodResourceInfo, nodeName string) (*PredicateError, *clustersnapshot.PodResourceInfo)
}
