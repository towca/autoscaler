package dynamicresources

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/uuid"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

// SanitizedNodeDynamicResources returns a deep copy of the provided NodeDynamicResources where:
// - NodeName pointers in all DRA objects are updated to the provided nodeName.
// - Names of all DRA objects get the provided nameSuffix appended.
// - UIDs of all DRA objects are randomized.
//
// This needs to be done anytime we want to add a "copy" of some Node (and so also of its NodeDynamicResources)
// to ClusterSnapshot.
func SanitizedNodeDynamicResources(ndr schedulerframework.NodeDynamicResources, nodeName string, nameSuffix string) schedulerframework.NodeDynamicResources {
	sanitizedNdr := ndr.DeepCopy()
	for _, claim := range sanitizedNdr.ResourceSlices {
		claim.Name = fmt.Sprintf("%s-%s", claim.Name, nameSuffix)
		claim.UID = uuid.NewUUID()
		claim.NodeName = nodeName
	}
	return sanitizedNdr
}
