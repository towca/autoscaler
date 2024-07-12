package clustersnapshot

import apiv1 "k8s.io/api/core/v1"

func FilterByPod(pods []*PodResourceInfo, filterFn func(pod *apiv1.Pod) bool) []*PodResourceInfo {
	var result []*PodResourceInfo
	for _, pod := range pods {
		if filterFn(pod.Pod) {
			result = append(result, pod)
		}
	}
	return result
}

func ApplyToPods(pods []*PodResourceInfo, applyFn func(pod *apiv1.Pod) *apiv1.Pod) []*PodResourceInfo {
	var result []*PodResourceInfo
	for _, pod := range pods {
		result = append(result, &PodResourceInfo{
			Pod:                     applyFn(pod.Pod),
			DynamicResourceRequests: pod.DynamicResourceRequests,
		})
	}
	return result
}

func ToPods(pods []*PodResourceInfo) []*apiv1.Pod {
	var result []*apiv1.Pod
	for _, pod := range pods {
		result = append(result, pod.Pod)
	}
	return result
}
