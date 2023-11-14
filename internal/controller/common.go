package controller

import (
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NineResourceName(cluster *ninev1alpha1.NineCluster) string {
	return cluster.Name + ninev1alpha1.ClusterNameSuffix
}

func MinioNewUserName(cluster *ninev1alpha1.NineCluster) string {
	return cluster.Name + ninev1alpha1.ClusterNameSuffix + "-user"
}

func NineConstructLabels(cluster *ninev1alpha1.NineCluster) map[string]string {
	return map[string]string{
		"cluster": cluster.Name,
		"app":     ninev1alpha1.ClusterSign,
	}
}

func NineObjectMeta(cluster *ninev1alpha1.NineCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      NineResourceName(cluster),
		Namespace: cluster.Namespace,
		Labels:    NineConstructLabels(cluster),
	}
}
