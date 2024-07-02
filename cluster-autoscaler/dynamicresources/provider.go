package dynamicresources

import (
	resourceapi "k8s.io/api/resource/v1alpha2"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	resourceapilisters "k8s.io/client-go/listers/resource/v1alpha2"
	"k8s.io/klog/v2"
)

// Provider provides DRA-related objects. Zero-value Provider object provides no objects, it can be used e.g. in tests.
type Provider struct {
	resourceClaims          resourceapilisters.ResourceClaimLister
	resourceClaimParameters resourceapilisters.ResourceClaimParametersLister
	resourceSlices          resourceapilisters.ResourceSliceLister
}

func NewProvider(informerFactory informers.SharedInformerFactory) *Provider {
	return &Provider{
		resourceClaims:          informerFactory.Resource().V1alpha2().ResourceClaims().Lister(),
		resourceClaimParameters: informerFactory.Resource().V1alpha2().ResourceClaimParameters().Lister(),
		resourceSlices:          informerFactory.Resource().V1alpha2().ResourceSlices().Lister(),
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

	params, err := p.resourceClaimParameters.List(labels.Everything())
	if err != nil {
		return Snapshot{}, err
	}
	paramsMap := make(map[objectRef]*resourceapi.ResourceClaimParameters)
	for _, param := range params {
		paramsMap[objectRef{name: param.Name, namespace: param.Namespace}] = param
	}

	slices, err := p.resourceSlices.List(labels.Everything())
	if err != nil {
		return Snapshot{}, err
	}
	slicesMap := make(map[string][]*resourceapi.ResourceSlice)
	for _, slice := range slices {
		if slice.NodeName == "" {
			klog.Warningf("DRA: ignoring non-Node-local ResourceSlice %s/%s", slice.Namespace, slice.Name)
			continue
		}
		slicesMap[slice.NodeName] = append(slicesMap[slice.NodeName], slice)
	}

	return Snapshot{
		resourceClaimsByRef:          claimMap,
		resourceClaimParametersByRef: paramsMap,
		resourceSlicesByNodeName:     slicesMap,
	}, nil
}
