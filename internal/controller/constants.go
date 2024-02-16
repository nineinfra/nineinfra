package controller

const (
	DefaultGloableNameSuffix     = "nineinfra"
	DefaultStorageClass          = "nineinfra-default"
	DefaultShuffleDiskSize       = "200Gi"
	DefaultShuffleDiskMountPath  = "/opt/spark/mnt/dir1"
	DefualtPasswordMD5Salt       = "nineinfra"
	DefaultHeadlessSvcNameSuffix = "-headless"
	DefaultClusterLabelKey       = "cluster"
	DefaultAppLabelKey           = "app"
	DefaultDiskNum               = 4
	DefaultMaxWaitSeconds        = 600
	// DefaultClusterDomainName is the default domain name key for the k8s cluster
	DefaultClusterDomainName = "clusterDomain"
	// DefaultClusterDomain is the default domain name value for the k8s cluster
	DefaultClusterDomain = "cluster.local"
)

const (
	DefaultMinioNameSuffix = "-minio"
	MinioErrorNotExist     = "The specified key does not exist"
)

const (
	PGResourceNameSuffix       = "-pg"
	DefaultPGSuperUserName     = "postgres"
	DefaultPGSuperUserPassword = "postgres"
	DefaultPGServerPort        = 5432
	PGErrorDuplicateKey        = "duplicate key value violates unique constraint"
)

const (
	DefaultKyuubiAuthUserName = "kyuubi"
	DefaultKyuubiAuthPassword = "kyuubi"
	DefaultKyuubiAuthDatabase = "kyuubi"
	DefaultKyuubiZKNamespace  = "kyuubi"
)

const (
	ZKResourceNameSuffix        = "-zookeeper"
	DefaultZookeeperClusterSign = "zookeeper"
	DefaultZKClientPortName     = "client"
)

const (
	HdfsResourceNameSuffix = "-hdfs"
	DefaultHdfsClusterSign = "hdfs"
	HdfsRoleNameNode       = "namenode"
)

const (
	MetastoreResourceNameSuffix = "-metastore"
	DefaultMetastoreClusterSign = "metastore"
)
