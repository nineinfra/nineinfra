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

const ClusterNameSuffix = "-metastore"
const ClusterSign = "metastore"

type ClusterType string

// Different types of clusters.
const (
	DatabaseClusterType ClusterType = "database"
	MinioClusterType    ClusterType = "minio"
	HdfsClusterType     ClusterType = "hdfs"
)

// Definitions to manage status conditions
const (
	// available
	StateAvailable = "Available"
	// failed
	StateFailed = "Failed"
)

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

type ResourceConfig struct {
	// The replicas of the kyuubi cluster workload.
	Replicas int32 `json:"replicas"`
}

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

type MinioCluster struct {
	// Endpoint of minio.Default is the service called minio.
	Endpoint string `json:"endpoint"`
	// Access Key
	AccessKey string `json:"accessKey"`
	// Secret Key
	SecretKey string `json:"secretKey"`
	// SSL enabled
	SSLEnabled string `json:"sslEnabled"`
	//Path style access
	PathStyleAccess string `json:"pathStyleAccess"`
}

type HdfsCluster struct {
	// +optional
	// HDFS core-site.xml.The type of the value must be string
	CoreSite map[string]string `json:"coreSite,omitempty"`
	// +optional
	// HDFS hdfs-site.xml.The type of the value must be string
	HdfsSite map[string]string `json:"hdfsSite,omitempty"`
}

type ClusterRef struct {
	// Name is the name of the referenced cluster
	Name string `json:"name"`
	// +kubebuilder:validation:Enum={database,minio,hdfs}
	Type ClusterType `json:"type"`
	// ClusterInfo is the detail info of the cluster of the clustertype
	// +optional
	// Database cluster infos referenced by the Metastore cluster
	Database DatabaseCluster `json:"database"`
	// +optional
	// Minio cluster infos referenced by the Metastore cluster
	Minio MinioCluster `json:"minio"`
	// +optional
	// HDFS cluster infos referenced by the Metastore cluster
	Hdfs HdfsCluster `json:"hdfs"`
}

// MetastoreClusterSpec defines the desired state of MetastoreCluster
type MetastoreClusterSpec struct {
	//Metastore version
	MetastoreVersion string `json:"metastoreVersion"`
	//Metastore image info
	MetastoreImage ImageConfig `json:"metastoreImage"`
	//Metastore resouce configuration
	MetastoreResource ResourceConfig `json:"metastoreResource"`
	//Metastore configurations.These cofigurations will be injected into the hive-site.xml.The type of the value must be string
	MetastoreConf map[string]string `json:"metastoreConf"`
	//Clusters referenced by Metastore
	ClusterRefs []ClusterRef `json:"clusterRefs"`
}

type ExposedType string

const (
	ExposedRest       ExposedType = "rest"
	ExposedThriftHttp ExposedType = "thrift-http"
)

type ExposedInfo struct {
	//Exposed name.
	Name string `json:"name"`
	//Exposed type. Support REST and THRIFT_BINARY
	ExposedType ExposedType `json:"exposedType"`
	//Exposed service name
	ServiceName string `json:"serviceName"`
	//Exposed service port info
	ServicePort corev1.ServicePort `json:"servicePort"`
}

// MetastoreClusterStatus defines the observed state of MetastoreCluster
type MetastoreClusterStatus struct {
	// Represents the observations of a NineCluster's current state.
	// MetastoreCluster.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// MetastoreCluster.status.conditions.status are one of True, False, Unknown.
	// MetastoreCluster.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// MetastoreCluster.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
	ExposedInfos []ExposedInfo      `json:"exposedInfos"`
	CreationTime metav1.Time        `json:"creationTime"`
	UpdateTime   metav1.Time        `json:"updateTime"`
	Conditions   []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MetastoreCluster is the Schema for the metastoreclusters API
type MetastoreCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetastoreClusterSpec   `json:"spec,omitempty"`
	Status MetastoreClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MetastoreClusterList contains a list of MetastoreCluster
type MetastoreClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetastoreCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetastoreCluster{}, &MetastoreClusterList{})
}
