package controller

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"

	kov1alpha1 "github.com/nineinfra/kyuubi-operator/api/v1alpha1"
	koversioned "github.com/nineinfra/kyuubi-operator/client/clientset/versioned"
	koscheme "github.com/nineinfra/kyuubi-operator/client/clientset/versioned/scheme"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
)

func (r *NineClusterReconciler) getKyuubiReplicas(ctx context.Context, cluster *ninev1alpha1.NineCluster, kyuubi ninev1alpha1.ClusterInfo) int32 {
	var replicas = kyuubi.Resource.Replicas
	if IsKyuubiNeedHA(cluster) {
		replicas = 2
	} else {
		if replicas == 0 {
			replicas = 1
		}
	}
	return replicas
}

func (r *NineClusterReconciler) constructMetastoreConf(ctx context.Context, cluster *ninev1alpha1.NineCluster, kyuubi ninev1alpha1.ClusterInfo) (map[string]string, error) {
	metastoreExposedInfo, err := r.getMetastoreExposedInfo(ctx, cluster)
	if err != nil {
		LogError(ctx, err, "get metastore exposed info failed")
		return nil, err
	}

	hiveSite := r.constructHiveSite(ctx, cluster)
	hiveSite["hive.metastore.uris"] = fmt.Sprintf("thrift://%s.%s.svc.%s:%d", metastoreExposedInfo.ServiceName, cluster.Namespace, GetClusterDomain(cluster, ninev1alpha1.MetaStoreClusterType), metastoreExposedInfo.ServicePort.Port)

	return hiveSite, nil
}

func (r *NineClusterReconciler) constructKyuubiConf(ctx context.Context, cluster *ninev1alpha1.NineCluster, kyuubi ninev1alpha1.ClusterInfo) (map[string]string, error) {
	authConfig := r.getAuthConfig(kyuubi)
	err := r.configJdbcAuth(ctx, cluster, kyuubi, authConfig)
	if err != nil {
		LogError(ctx, err, "config kyuubi auth failed")
		return nil, err
	}

	var kyuubiConf map[string]string
	if authConfig.AuthType == ninev1alpha1.ClusterAuthTypeJDBC {
		kyuubiConf = map[string]string{
			"kyuubi.kubernetes.namespace":             cluster.Namespace,
			"kyuubi.authentication":                   "JDBC",
			"kyuubi.authentication.jdbc.driver.class": "org.postgresql.Driver",
			"kyuubi.authentication.jdbc.url":          r.BuildPGJdbcWithCluster(cluster, "", "", DefaultKyuubiAuthDatabase),
			"kyuubi.authentication.jdbc.user":         DefaultKyuubiAuthUserName,
			"kyuubi.authentication.jdbc.password":     DefaultKyuubiAuthPassword,
			"kyuubi.authentication.jdbc.query":        `SELECT 1 FROM users WHERE username=${user} AND passwd=MD5(CONCAT('nineinfra',${password}))`,
		}
	} else {
		kyuubiConf = map[string]string{
			"kyuubi.kubernetes.namespace": cluster.Namespace,
		}
	}

	if IsKyuubiNeedHA(cluster) {
		zkIpAndPorts, err := r.getZookeeperExposedInfo(ctx, cluster)
		if err != nil {
			LogError(ctx, err, "get zookeeper exposed info failed")
			return nil, err
		}
		kyuubiConf["kyuubi.ha.namespace"] = DefaultKyuubiZKNamespace
		kyuubiConf["kyuubi.ha.client.class"] = "org.apache.kyuubi.ha.client.zookeeper.ZookeeperDiscoveryClient"

		kyuubiConf["kyuubi.ha.addresses"] = strings.Join(zkIpAndPorts, ",")
		//kyuubiConf["kyuubi.frontend.bind.host"] = os.Getenv("POD_IP")
	}

	for k, v := range kyuubi.Configs.Conf {
		kyuubiConf[k] = v
	}

	return kyuubiConf, nil
}

func (r *NineClusterReconciler) constructSparkConf(ctx context.Context, cluster *ninev1alpha1.NineCluster, kyuubi ninev1alpha1.ClusterInfo) (map[string]string, error) {
	sparkConf := make(map[string]string, 0)
	switch GetClusterStorage(cluster) {
	case ninev1alpha1.NineClusterStorageMinio:
		minioExposedInfo, err := r.getMinioExposedInfo(ctx, cluster)
		if err != nil {
			LogError(ctx, err, "get minio exposed info failed")
			return nil, err
		}
		sparkConf = map[string]string{
			"spark.hadoop.fs.s3a.access.key":             minioExposedInfo.AccessKey,
			"spark.hadoop.fs.s3a.secret.key":             minioExposedInfo.SecretKey,
			"spark.hadoop.fs.s3a.path.style.access":      "true",
			"spark.hadoop.fs.s3a.connection.ssl.enabled": "false",
			"spark.hadoop.fs.s3a.endpoint":               minioFullEndpoint(minioExposedInfo.Endpoint, false),
		}
	case ninev1alpha1.NineClusterStorageHdfs:
		//Todo
	}
	spark := GetRefClusterInfo(cluster, ninev1alpha1.SparkClusterType)

	for k, v := range spark.Configs.Conf {
		sparkConf[k] = v
	}
	//Currently,rss not supported,so one shuffle disk should be guaranteed
	if _, ok := sparkConf["spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.options.sizeLimit"]; !ok {
		sparkConf["spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.options.claimName"] = "OnDemand"
		sparkConf["spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.options.storageClass"] = GetStorageClassName(&kyuubi)
		sparkConf["spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.options.sizeLimit"] = DefaultShuffleDiskSize
		sparkConf["spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.mount.path"] = DefaultShuffleDiskMountPath
		sparkConf["spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.mount.readOnly"] = "false"
	}

	return sparkConf, nil
}

func (r *NineClusterReconciler) getAuthConfig(kyuubi ninev1alpha1.ClusterInfo) ninev1alpha1.AuthConfig {
	if kyuubi.Configs.Auth.AuthType == "" {
		return ninev1alpha1.AuthConfig{
			AuthType: ninev1alpha1.ClusterAuthTypeJDBC,
			UserName: DefaultKyuubiAuthUserName,
			Password: DefaultKyuubiAuthPassword,
		}
	}
	return kyuubi.Configs.Auth
}

func (r *NineClusterReconciler) configJdbcAuth(ctx context.Context, cluster *ninev1alpha1.NineCluster, kyuubi ninev1alpha1.ClusterInfo, authConfig ninev1alpha1.AuthConfig) error {
	if authConfig.AuthType == ninev1alpha1.ClusterAuthTypeJDBC {
		dbUser := DefaultKyuubiAuthUserName
		dbPassword := DefaultKyuubiAuthPassword
		dbName := DefaultKyuubiAuthDatabase
		err := r.createDatabase(ctx, cluster, dbUser, dbPassword, dbName)
		if err != nil {
			return err
		}

		sqlStr := `CREATE TABLE IF NOT EXISTS users (username TEXT PRIMARY KEY,passwd TEXT)`
		err = r.executeSql(ctx, cluster, dbUser, dbPassword, dbName, sqlStr)
		if err != nil {
			return err
		}
		sqlStr = `INSERT INTO users (username, passwd) VALUES ($1, $2)`
		passwdMD5 := md5.New()
		passwdMD5.Write([]byte(fmt.Sprintf("%s%s", DefualtPasswordMD5Salt, authConfig.Password)))
		passwd := hex.EncodeToString(passwdMD5.Sum(nil))
		sqlArgs := []any{authConfig.UserName, passwd}
		err = r.executeSql(ctx, cluster, dbUser, dbPassword, dbName, sqlStr, sqlArgs...)
		if err != nil && !strings.Contains(err.Error(), PGErrorDuplicateKey) {
			return err
		}
	}
	return nil
}

func (r *NineClusterReconciler) constructKyuubiClusterRefs(ctx context.Context, cluster *ninev1alpha1.NineCluster, kyuubi ninev1alpha1.ClusterInfo) ([]kov1alpha1.ClusterRef, error) {
	metastoreConf, err := r.constructMetastoreConf(ctx, cluster, kyuubi)
	if err != nil {
		return nil, err
	}

	clusterRefs := []kov1alpha1.ClusterRef{
		{
			Name: "metastore",
			Type: "metastore",
			Metastore: kov1alpha1.MetastoreCluster{
				HiveSite: metastoreConf,
			},
		},
	}
	var sparkConf map[string]string
	switch cluster.Spec.Type {
	case ninev1alpha1.NineClusterTypeBatch:
		sparkConf, err = r.constructSparkConf(ctx, cluster, kyuubi)
		if err != nil {
			return nil, err
		}
		LogInfo(ctx, fmt.Sprintf("sparkConf:%v\n", sparkConf))
		spark := GetRefClusterInfo(cluster, ninev1alpha1.SparkClusterType)
		clusterRefs = append(clusterRefs, kov1alpha1.ClusterRef{
			Name: "spark",
			Type: "spark",
			Spark: kov1alpha1.SparkCluster{
				SparkMaster: "k8s",
				SparkImage: kov1alpha1.ImageConfig{
					Repository: spark.Configs.Image.Repository,
					Tag:        spark.Configs.Image.Tag,
					PullPolicy: spark.Configs.Image.PullPolicy,
				},
				SparkNamespace: cluster.Namespace,
				SparkDefaults:  sparkConf,
			},
		})
	case ninev1alpha1.NineClusterTypeStream:
		//Todo support flink
	}

	switch GetClusterStorage(cluster) {
	case ninev1alpha1.NineClusterStorageHdfs:
		clusterRefs = append(clusterRefs, kov1alpha1.ClusterRef{
			Name: "hdfs",
			Type: "hdfs",
			Hdfs: kov1alpha1.HdfsCluster{
				CoreSite: r.constructCoreSite(ctx, cluster),
				HdfsSite: r.constructHdfsSite(ctx, cluster),
			},
		})
	}

	return clusterRefs, nil
}

func (r *NineClusterReconciler) constructKyuubiCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, kyuubi ninev1alpha1.ClusterInfo) (*kov1alpha1.KyuubiCluster, error) {
	kyuubiConf, err := r.constructKyuubiConf(ctx, cluster, kyuubi)
	if err != nil {
		return nil, err
	}

	clusterRefs, err := r.constructKyuubiClusterRefs(ctx, cluster, kyuubi)
	if err != nil {
		return nil, err
	}
	kyuubiDesired := &kov1alpha1.KyuubiCluster{
		ObjectMeta: NineObjectMeta(cluster),
		//Todo,here should be a template instead of hardcoding?
		Spec: kov1alpha1.KyuubiClusterSpec{
			KyuubiVersion: kyuubi.Version,
			KyuubiResource: kov1alpha1.ResourceConfig{
				Replicas: r.getKyuubiReplicas(ctx, cluster, kyuubi),
			},
			KyuubiImage: kov1alpha1.ImageConfig{
				Repository: kyuubi.Configs.Image.Repository,
				Tag:        kyuubi.Configs.Image.Tag,
				PullPolicy: kyuubi.Configs.Image.PullPolicy,
			},
			KyuubiConf:  kyuubiConf,
			ClusterRefs: clusterRefs,
		},
	}

	if err := ctrl.SetControllerReference(cluster, kyuubiDesired, r.Scheme); err != nil {
		return nil, err
	}

	return kyuubiDesired, nil
}

func (r *NineClusterReconciler) reconcileKyuubiCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, kyuubi ninev1alpha1.ClusterInfo, logger logr.Logger) error {
	desiredKyuubi, err := r.constructKyuubiCluster(ctx, cluster, kyuubi)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Wait for other resource to construct KyuubiCluster...")
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to construct KyuubiCluster")
		return err
	}

	metav1.AddToGroupVersion(koscheme.Scheme, kov1alpha1.GroupVersion)
	utilruntime.Must(kov1alpha1.AddToScheme(koscheme.Scheme))

	config, err := GetK8sClientConfig()
	if err != nil {
		return err
	}

	kc, err := koversioned.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = kc.KyuubiV1alpha1().KyuubiClusters(cluster.Namespace).Get(context.TODO(), NineResourceName(cluster), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		logger.Info("Start to create a new KyuubiCluster...")
		_, err := kc.KyuubiV1alpha1().KyuubiClusters(cluster.Namespace).Create(context.TODO(), desiredKyuubi, metav1.CreateOptions{})
		if err != nil {
			//Todo,may be exist already due to the go routine parallel exec
			return err
		}
	}
	logger.Info("Reconcile a KyuubiCluster successfully")
	return nil
}
