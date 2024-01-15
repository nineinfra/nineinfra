package controller

import (
	"errors"
	"fmt"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"strconv"
	"strings"
)

// GenUniqueName4Cluster format: DefaultGloableNameSuffix-namespace-ninename-clustertype
func GenUniqueName4Cluster(cluster *ninev1alpha1.NineCluster, clusterType ninev1alpha1.ClusterType) string {
	return fmt.Sprintf("%s-%s-%s-%s", DefaultGloableNameSuffix, cluster.Namespace, cluster.Name, clusterType)
}

func NineResourceName(cluster *ninev1alpha1.NineCluster, suffixs ...string) string {
	return cluster.Name + ninev1alpha1.ClusterNameSuffix + strings.Join(suffixs, "-")
}

func MinioNewUserName(cluster *ninev1alpha1.NineCluster) string {
	return cluster.Name + ninev1alpha1.ClusterNameSuffix + DefaultMinioNameSuffix + "-user"
}

func MinioConfigName(cluster *ninev1alpha1.NineCluster) string {
	return cluster.Name + ninev1alpha1.ClusterNameSuffix + DefaultMinioNameSuffix + "-config"
}

func PGInitDBUserSecretName(cluster *ninev1alpha1.NineCluster) string {
	return cluster.Name + ninev1alpha1.ClusterNameSuffix + PGResourceNameSuffix + "-user"
}

func PGSuperUserSecretName(cluster *ninev1alpha1.NineCluster) string {
	return cluster.Name + ninev1alpha1.ClusterNameSuffix + PGResourceNameSuffix + "-superuser"
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
	return "", errors.New(fmt.Sprintf("no supported olap found,[%s] supported now", ninev1alpha1.NineInfraSupportedOlapList))
}

func CheckStorageSupported(c ninev1alpha1.ClusterStorage) bool {
	for _, v := range ninev1alpha1.NineInfraSupportedStorageList {
		if c == v {
			return true
		}
	}
	return false
}

func GetClusterStorage(cluster *ninev1alpha1.NineCluster) (ninev1alpha1.ClusterStorage, error) {
	if cluster.Spec.Features != nil {
		if value, ok := cluster.Spec.Features[ninev1alpha1.NineClusterFeatureStorage]; ok {
			s := ninev1alpha1.ClusterStorage(value)
			if CheckStorageSupported(s) {
				return s, nil
			}
		}
	}
	return ninev1alpha1.NineClusterStorageMinio, nil
}

func IsKyuubiNeedHA(cluster *ninev1alpha1.NineCluster) bool {
	if cluster.Spec.Features != nil {
		if value, ok := cluster.Spec.Features[ninev1alpha1.NineClusterFeatureKyuubiHA]; ok {
			ha, err := strconv.ParseBool(value)
			if err != nil {
				return false
			}
			return ha
		}
	}
	return false
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
	storage, _ := GetClusterStorage(cluster)
	iskyuubiha := IsKyuubiNeedHA(cluster)
	var weNeedClusterTypes = map[ninev1alpha1.ClusterType]bool{}
	for _, v := range ninev1alpha1.NineDatahouseClusterset {
		weNeedClusterTypes[v.Type] = true
	}
	if cluster.Spec.Type == ninev1alpha1.NineClusterTypeBatch {
		weNeedClusterTypes[ninev1alpha1.SparkClusterType] = true
	}

	if iskyuubiha || storage == ninev1alpha1.NineClusterStorageHdfs {
		weNeedClusterTypes[ninev1alpha1.ZookeeperClusterType] = true
	}

	if storage == ninev1alpha1.NineClusterStorageMinio {
		weNeedClusterTypes[ninev1alpha1.MinioClusterType] = true
	}

	if olap == ninev1alpha1.DorisClusterType {
		weNeedClusterTypes[ninev1alpha1.DorisClusterType] = true
		weNeedClusterTypes[ninev1alpha1.DorisFEClusterType] = true
		weNeedClusterTypes[ninev1alpha1.DorisBEClusterType] = true
	}
	var userSpecifyClusterTypes = map[ninev1alpha1.ClusterType]bool{}
	if cluster.Spec.ClusterSet != nil {
		for _, v := range cluster.Spec.ClusterSet {
			userSpecifyClusterTypes[v.Type] = true
		}
	}
	if cluster.Spec.ClusterSet == nil {
		cluster.Spec.ClusterSet = make([]ninev1alpha1.ClusterInfo, 0)
	}
	for _, v := range ninev1alpha1.NineDatahouseFullClusterset {
		if _, custom := weNeedClusterTypes[v.Type]; custom {
			if _, specify := userSpecifyClusterTypes[v.Type]; !specify {
				cluster.Spec.ClusterSet = append(cluster.Spec.ClusterSet, v)
			}
		}
	}

	return nil
}
