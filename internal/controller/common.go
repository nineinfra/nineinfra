package controller

import (
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"strings"
)

func NineResourceName(cluster *ninev1alpha1.NineCluster, suffixs ...string) string {
	return cluster.Name + ninev1alpha1.ClusterNameSuffix + strings.Join(suffixs, "-")
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

func NineObjectMeta(cluster *ninev1alpha1.NineCluster, suffixs ...string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      NineResourceName(cluster) + strings.Join(suffixs, "-"),
		Namespace: cluster.Namespace,
		Labels:    NineConstructLabels(cluster),
	}
}

func GetK8sClientConfig() (*rest.Config, error) {
	//Todo,support run out of the k8s cluster
	config, err := rest.InClusterConfig()
	return config, err
}

func GetRefClusterInfo(cluster *ninev1alpha1.NineCluster, clusterType ninev1alpha1.ClusterType) *ninev1alpha1.ClusterInfo {
	for _, v := range cluster.Spec.ClusterSet {
		if v.Type == clusterType {
			return &v
		}
	}
	return nil
}

func GetDefaultRefClusterInfo(clusterType ninev1alpha1.ClusterType) *ninev1alpha1.ClusterInfo {
	for _, v := range ninev1alpha1.NineDatahouseClusterset {
		if v.Type == clusterType {
			return &v
		}
	}
	return nil
}

func GetStorageClassName(cluster *ninev1alpha1.ClusterInfo) string {
	if cluster.Resource.StorageClass != "" {
		return cluster.Resource.StorageClass
	}
	return DefaultStorageClass
}
