package dynamicresources

import (
	resourceapi "k8s.io/api/resource/v1alpha3"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	resourceapilisters "k8s.io/client-go/listers/resource/v1alpha3"
	"k8s.io/klog/v2"
)

// Provider provides DRA-related objects. Zero-value Provider object provides no objects, it can be used e.g. in tests.
type Provider struct {
	resourceClaims resourceapilisters.ResourceClaimLister
	resourceSlices resourceapilisters.ResourceSliceLister
}

func NewProvider(informerFactory informers.SharedInformerFactory) *Provider {
	return &Provider{
		resourceClaims: informerFactory.Resource().V1alpha3().ResourceClaims().Lister(),
		resourceSlices: informerFactory.Resource().V1alpha3().ResourceSlices().Lister(),
	}
}

func (p *Provider) Snapshot() (Snapshot, error) {
	// Zero value is safe to use as a no-op lister in tests.
	if *p == (Provider{}) {
		return Snapshot{}, nil
	}

	claims, err := p.resourceClaims.List(labels.Everything())
	if err != nil {
		return Snapshot{}, err
	}
	claimMap := make(map[objectRef]*resourceapi.ResourceClaim)
	for _, claim := range claims {
		claimMap[objectRef{name: claim.Name, namespace: claim.Namespace}] = claim
	}

	slices, err := p.resourceSlices.List(labels.Everything())
	if err != nil {
		return Snapshot{}, err
	}
	slicesMap := make(map[string][]*resourceapi.ResourceSlice)
	for _, slice := range slices {
		if slice.Spec.NodeName == "" {
			klog.Warningf("DRA: ignoring non-Node-local ResourceSlice %s/%s", slice.Namespace, slice.Name)
			continue
		}
		slicesMap[slice.Spec.NodeName] = append(slicesMap[slice.Spec.NodeName], slice)
	}

	return Snapshot{
		resourceClaimsByRef:      claimMap,
		resourceSlicesByNodeName: slicesMap,
	}, nil
}
