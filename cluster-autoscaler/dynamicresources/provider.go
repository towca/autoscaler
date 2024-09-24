package dynamicresources

import (
	resourceapi "k8s.io/api/resource/v1alpha3"
	"k8s.io/client-go/informers"
)

// Provider provides DRA-related objects. Zero-value Provider object provides no objects, it can be used e.g. in tests.
type Provider struct {
	resourceClaims resourceClaimLister
	resourceSlices resourceSliceLister
	deviceClasses  deviceClassLister
}

func NewProviderFromInformers(informerFactory informers.SharedInformerFactory) *Provider {
	claims := &resourceClaimApiLister{apiLister: informerFactory.Resource().V1alpha3().ResourceClaims().Lister()}
	slices := &resourceSliceApiLister{apiLister: informerFactory.Resource().V1alpha3().ResourceSlices().Lister()}
	devices := &deviceClassApiLister{apiLister: informerFactory.Resource().V1alpha3().DeviceClasses().Lister()}
	return NewProvider(claims, slices, devices)
}

func NewProvider(claims resourceClaimLister, slices resourceSliceLister, classes deviceClassLister) *Provider {
	return &Provider{
		resourceClaims: claims,
		resourceSlices: slices,
		deviceClasses:  classes,
	}
}

func (p *Provider) Snapshot() (Snapshot, error) {
	// Zero value is safe to use as a no-op lister in tests.
	if *p == (Provider{}) {
		return Snapshot{}, nil
	}

	claims, err := p.resourceClaims.List()
	if err != nil {
		return Snapshot{}, err
	}
	claimMap := make(map[ResourceClaimRef]*resourceapi.ResourceClaim)
	for _, claim := range claims {
		claimMap[ResourceClaimRef{Name: claim.Name, Namespace: claim.Namespace}] = claim
	}

	slices, err := p.resourceSlices.List()

	if err != nil {
		return Snapshot{}, err
	}
	slicesMap := make(map[string][]*resourceapi.ResourceSlice)
	var nonNodeLocalSlices []*resourceapi.ResourceSlice
	for _, slice := range slices {
		if slice.Spec.NodeName == "" {
			nonNodeLocalSlices = append(nonNodeLocalSlices, slice)
		} else {
			slicesMap[slice.Spec.NodeName] = append(slicesMap[slice.Spec.NodeName], slice)
		}
	}

	classes, err := p.deviceClasses.List()
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		resourceClaimsByRef:        claimMap,
		resourceSlicesByNodeName:   slicesMap,
		NonNodeLocalResourceSlices: nonNodeLocalSlices,
		DeviceClasses:              classes,
	}, nil
}
