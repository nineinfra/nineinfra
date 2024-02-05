/*
Copyright 2023 nineinfra.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const ClusterNameSuffix = "-nine"
const ClusterSign = "nine"
const DataHouseDir = "/nineinfra/datahouse"
const DefaultMinioBucket = "nineinfra"
const DefaultMinioDataHouseFolder = "datahouse/"

// DefaultDbType ,the default value of the DbType, support mysql and postgres
const DefaultDbType = "postgres"

// NineClusterType describes the type of the nineclusters
type NineClusterType string

// Different types of nineclusters.
const (
	NineClusterTypeStream NineClusterType = "stream"
	NineClusterTypeBatch  NineClusterType = "batch"
)

var NineClusterTypeSupportedList = []NineClusterType{NineClusterTypeStream, NineClusterTypeBatch}

const (
	StateAvailable = "Available"
	StateDeploying = "Deploying"
	StateFailed    = "Failed"
)

type ClusterType string

// Different types of clusters.
const (
	KyuubiClusterType          ClusterType = "kyuubi"
	DorisClusterType           ClusterType = "doris"
	DorisFEClusterType         ClusterType = "doris-fe"
	DorisBEClusterType         ClusterType = "doris-be"
	SparkClusterType           ClusterType = "spark"
	MetaStoreClusterType       ClusterType = "metastore"
	DatabaseClusterType        ClusterType = "database"
	MinioClusterType           ClusterType = "minio"
	ZookeeperClusterType       ClusterType = "zookeeper"
	HdfsClusterType            ClusterType = "hdfs"
	HdfsNameNodeClusterType    ClusterType = "namenode"
	HdfsDataNodeClusterType    ClusterType = "datanode"
	HdfsJournalNodeClusterType ClusterType = "journalnode"
)

const (
	DbTypePostgres = "postgres"
	DbTypeMysql    = "mysql"
)

type ClusterStorage string

const (
	NineClusterStorageMinio ClusterStorage = "minio"
	NineClusterStorageHdfs  ClusterStorage = "hdfs"
)
const (
	NineClusterFeatureOlap          = "olap"
	NineClusterFeatureStorage       = "storage"
	NineClusterFeatureCustomStorage = "custom-storage"
	NineClusterFeatureKyuubiHA      = "kyuubi-ha"
)

const (
	ClusterAuthTypeSimple = "simple"
	ClusterAuthTypeJDBC   = "JDBC"
)

var NineInfraSupportedOlapList = []ClusterType{DorisClusterType}
var NineInfraSupportedStorageList = []ClusterStorage{NineClusterStorageMinio, NineClusterStorageHdfs}

type DatabaseCluster struct {
	// connection Url of the database.such as jdbc:mysql://mysql:3306/metastore
	ConnectionUrl string `json:"connectionUrl"`
	//Db type.Specified the driver name.Support mysql,postgres
	DbType string `json:"dbType"`
	// Username of the database.
	UserName string `json:"userName"`
	// password
	Password string `json:"password"`
}

type MinioExposedInfo struct {
	// Endpoint of the minio cluster.
	Endpoint string `json:"endpoint"`
	// Access key of the minio cluster.
	AccessKey string `json:"accessKey"`
	// Secret key of the minio cluster.
	SecretKey string `json:"secretKey"`
}

type HdfsExposedInfo struct {
	// DefaultFS fs.defaultFS of the hdfs.
	DefaultFS string `json:"defaultFS"`
	// HdfsSite hdfsSite.
	HdfsSite map[string]string `json:"hdfsSite"`
	// CoreSite coreSite.
	CoreSite map[string]string `json:"coreSite"`
}

type ResourceConfig struct {
	// The replicas of the cluster workload.Default value is 1
	// +optional
	Replicas int32 `json:"replicas"`
	// num of the disks. default value is 1
	// +optional
	Disks int32 `json:"disks"`
	// the storage class. default value is nineinfra-default
	// +optional
	StorageClass string `json:"storageClass"`
	// The resource requirements of the cluster workload.
	// +optional
	ResourceRequirements corev1.ResourceRequirements `json:"resourceRequirements"`
}

type ImageConfig struct {
	Repository string `json:"repository"`
	// Image tag. Usually the vesion of the kyuubi, default: `latest`.
	// +optional
	Tag string `json:"tag,omitempty"`
	// Image pull policy. One of `Always, Never, IfNotPresent`, default: `Always`.
	// +kubebuilder:default:=Always
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +optional
	PullPolicy string `json:"pullPolicy,omitempty"`
	// Secrets for image pull.
	// +optional
	PullSecrets string `json:"pullSecret,omitempty"`
}

type AuthConfig struct {
	// AuthType. The auth type of the cluster,default:ClusterAuthTypeSimple.
	// +optional
	AuthType string `json:"authType,omitempty"`
	// UserName for the cluster access.
	// +optional
	UserName string `json:"userName,omitempty"`
	// Password for the cluster access.
	// +optional
	Password string `json:"password,omitempty"`
}

type ClusterConfig struct {
	Image ImageConfig `json:"image,omitempty"`
	// Auth, Auth info of the cluster
	// +optional
	Auth AuthConfig `json:"auth,omitempty"`
	// Conf,k,v pairs will be into the main conf file
	// +optional
	Conf map[string]string `json:"conf,omitempty"`
	// K8sConf. k/v configs for the cluster in k8s.such as the cluster domain
	// +optional
	K8sConf map[string]string `json:"k8sConf,omitempty"`
}

type ClusterInfo struct {
	// Version of the cluster.
	Version string `json:"version"`
	// Type of the cluster.
	Type ClusterType `json:"type"`
	// Name. name of the cluster.
	// +optional
	Name string `json:"name"`
	// SubType,some type of cluster such as database has subtype,Support mysql,postgres
	// +optional
	SubType string `json:"subType,omitempty"`
	// Resource config of the cluster.
	// +optional
	Resource ResourceConfig `json:"resource,omitempty"`
	// +optional
	Configs ClusterConfig `json:"configs,omitempty"`
	// +optional
	ClusterRefs []ClusterType `json:"clusterRefs,omitempty"`
}

// NineClusterSpec defines the desired state of NineCluster
type NineClusterSpec struct {
	// Data Volume of the ninecluster. The unit of the data volume is Gi.
	DataVolume int `json:"dataVolume"`
	// Type of the ninecluster. default value is DataHouse.
	// +optional
	Type NineClusterType `json:"type,omitempty"`
	// Features of the ninecluster.
	// +optional
	Features map[string]string `json:"features,omitempty"`
	// Cluster set of the type of Nine
	// +optional
	ClusterSet []ClusterInfo `json:"clusterSet,omitempty"`
}

// NineClusterStatus defines the observed state of NineCluster
type NineClusterStatus struct {
	// Represents the observations of a NineCluster's current state.
	// NineCluster.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// NineCluster.status.conditions.status are one of True, False, Unknown.
	// NineCluster.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// NineCluster.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NineCluster is the Schema for the nineclusters API
type NineCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NineClusterSpec   `json:"spec"`
	Status NineClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NineClusterList contains a list of NineCluster
type NineClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NineCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NineCluster{}, &NineClusterList{})
}
