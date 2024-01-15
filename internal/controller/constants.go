package controller

const (
	DefaultGloableNameSuffix    = "nineinfra"
	DefaultStorageClass         = "nineinfra-default"
	DefaultShuffleDiskSize      = "200Gi"
	DefaultShuffleDiskMountPath = "/opt/spark/mnt/dir1"
	DefualtPasswordMD5Salt      = "nineinfra"
	DefaultHeadlessSvcNameSuffix = "-headless"
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
	ZKResourceNameSuffix    = "-zookeeper"
	DefaultZKClientPortName = "client"
)
