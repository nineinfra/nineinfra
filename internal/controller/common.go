package controller

import (
	"errors"
	"fmt"
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

func CheckOlapSupported(c ninev1alpha1.ClusterType) bool {
	for _, v := range ninev1alpha1.NineInfraSupportedOlapList {
		if c == v {
			return true
		}
	}
	return false
}

func CheckNineClusterTypeSupported(c ninev1alpha1.NineClusterType) bool {
	for _, v := range ninev1alpha1.NineClusterTypeSupportedList {
		if c == v {
			return true
		}
	}
	return false
}

func GetOlapClusterType(cluster *ninev1alpha1.NineCluster) (ninev1alpha1.ClusterType, error) {
	if value, ok := cluster.Spec.Features[ninev1alpha1.NineClusterFeatureOlap]; ok {
		c := ninev1alpha1.ClusterType(value)
		if CheckOlapSupported(c) {
			return c, nil
		}
	}
	return "", errors.New("no supported olap found")
}

func FillNineClusterType(cluster *ninev1alpha1.NineCluster) error {
	if cluster.Spec.Type == "" {
		cluster.Spec.Type = ninev1alpha1.NineClusterTypeBatch
	} else {
		if !CheckNineClusterTypeSupported(cluster.Spec.Type) {
			return errors.New(fmt.Sprintf("nine cluster type:%s not supported", cluster.Spec.Type))
		}
	}
	return nil
}

func FillClustersInfo(cluster *ninev1alpha1.NineCluster) error {
	olap, _ := GetOlapClusterType(cluster)
	if cluster.Spec.ClusterSet == nil {
		if olap == "" {
			cluster.Spec.ClusterSet = ninev1alpha1.NineDatahouseClusterset
		} else {
			cluster.Spec.ClusterSet = ninev1alpha1.NineDatahouseWithOLAPClusterset
		}
	} else {
		//check kyuubi,spark/flink,metastore and database
		var userClusterTypes = map[ninev1alpha1.ClusterType]bool{}
		for _, v := range cluster.Spec.ClusterSet {
			userClusterTypes[v.Type] = true
		}
		var defaultClusterSet []ninev1alpha1.ClusterInfo
		if olap == "" {
			defaultClusterSet = ninev1alpha1.NineDatahouseClusterset
		} else {
			defaultClusterSet = ninev1alpha1.NineDatahouseWithOLAPClusterset
		}
		for _, v := range defaultClusterSet {
			if _, ok := userClusterTypes[v.Type]; !ok {
				cluster.Spec.ClusterSet = append(cluster.Spec.ClusterSet, v)
			}
		}
	}
	return nil
}
