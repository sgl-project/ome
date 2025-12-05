package utils

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ObjectMetaPack contains metav1.ObjectMeta for various cases
type ObjectMetaPack struct {
	Normal metav1.ObjectMeta
	Pod    metav1.ObjectMeta
}
