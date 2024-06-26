package dynamicresources

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	listersv1alpha2 "k8s.io/client-go/listers/resource/v1alpha2"
)

type v1Alpha2Listers struct {
	resourceClaims          listersv1alpha2.ResourceClaimLister
	resourceClaimParameters listersv1alpha2.ResourceClaimParametersLister
	resourceSlices          listersv1alpha2.ResourceSliceLister
}

func (l v1Alpha2Listers) snapshot() (snapshotV1alpha2, error) {
	claims, err := l.resourceClaims.List(labels.Everything())
	if err != nil {
		return snapshotV1alpha2{}, err
	}
	params, err := l.resourceClaimParameters.List(labels.Everything())
	if err != nil {
		return snapshotV1alpha2{}, err
	}
	slices, err := l.resourceSlices.List(labels.Everything())
	if err != nil {
		return snapshotV1alpha2{}, err
	}
	return snapshotV1alpha2{
		resourceClaims:          claims,
		resourceClaimParameters: params,
		resourceSlices:          slices,
	}, nil
}

// Provider provides DRA-related objects.
type Provider struct {
	v1a2Listers v1Alpha2Listers
}

func NewProvider(informerFactory informers.SharedInformerFactory) *Provider {
	return &Provider{
		v1a2Listers: v1Alpha2Listers{
			resourceClaims:          informerFactory.Resource().V1alpha2().ResourceClaims().Lister(),
			resourceClaimParameters: informerFactory.Resource().V1alpha2().ResourceClaimParameters().Lister(),
			resourceSlices:          informerFactory.Resource().V1alpha2().ResourceSlices().Lister(),
		},
	}
}

func (m *Provider) Snapshot() (Snapshot, error) {
	v1a2Snapshot, err := m.v1a2Listers.snapshot()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		snapshotV1a2: v1a2Snapshot,
	}, nil
}
