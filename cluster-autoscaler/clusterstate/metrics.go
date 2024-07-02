package clusterstate

import (
	"reflect"

	"k8s.io/autoscaler/cluster-autoscaler/metrics"
)

// UpdateClusterStateMetrics updates metrics related to cluster state
func UpdateClusterStateMetrics(csr *ClusterStateRegistry) {
	if csr == nil || reflect.ValueOf(csr).IsNil() {
		return
	}
	metrics.UpdateClusterSafeToAutoscale(csr.IsClusterHealthy())
	readiness := csr.GetClusterReadiness()
	metrics.UpdateNodesCount(len(readiness.Ready), len(readiness.Unready), len(readiness.NotStarted), len(readiness.LongUnregistered), len(readiness.Unregistered))
}
