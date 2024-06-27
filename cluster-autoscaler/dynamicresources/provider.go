package dynamicresources

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	resourceapilisters "k8s.io/client-go/listers/resource/v1alpha2"
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
	params, err := p.resourceClaimParameters.List(labels.Everything())
	if err != nil {
		return Snapshot{}, err
	}
	slices, err := p.resourceSlices.List(labels.Everything())
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		resourceClaims:          claims,
		resourceClaimParameters: params,
		resourceSlices:          slices,
	}, nil
}
