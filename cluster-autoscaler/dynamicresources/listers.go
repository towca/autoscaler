package dynamicresources

import (
	resourceapi "k8s.io/api/resource/v1alpha3"
	"k8s.io/apimachinery/pkg/labels"
	resourceapilisters "k8s.io/client-go/listers/resource/v1alpha3"
)

type resourceClaimLister interface {
	List() ([]*resourceapi.ResourceClaim, error)
}

type resourceSliceLister interface {
	List() ([]*resourceapi.ResourceSlice, error)
}

type deviceClassLister interface {
	List() ([]*resourceapi.DeviceClass, error)
}

type resourceClaimApiLister struct {
	apiLister resourceapilisters.ResourceClaimLister
}

func (l *resourceClaimApiLister) List() ([]*resourceapi.ResourceClaim, error) {
	return l.apiLister.List(labels.Everything())
}

type resourceSliceApiLister struct {
	apiLister resourceapilisters.ResourceSliceLister
}

func (l *resourceSliceApiLister) List() (ret []*resourceapi.ResourceSlice, err error) {
	return l.apiLister.List(labels.Everything())
}

type deviceClassApiLister struct {
	apiLister resourceapilisters.DeviceClassLister
}

func (l *deviceClassApiLister) List() (ret []*resourceapi.DeviceClass, err error) {
	return l.apiLister.List(labels.Everything())
}
