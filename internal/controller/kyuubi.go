package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"strconv"

	kov1alpha1 "github.com/nineinfra/kyuubi-operator/api/v1alpha1"
	koversioned "github.com/nineinfra/kyuubi-operator/client/clientset/versioned"
	koscheme "github.com/nineinfra/kyuubi-operator/client/clientset/versioned/scheme"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
)

func (r *NineClusterReconciler) constructKyuubiCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, kyuubi ninev1alpha1.ClusterInfo) (*kov1alpha1.KyuubiCluster, error) {
	minioExposedInfo, err := r.getMinioExposedInfo(ctx, cluster)
	if err != nil {
		LogError(ctx, err, "get minio exposed info failed")
		return nil, err
	}
	metastoreExposedInfo, err := r.getMetastoreExposedInfo(ctx, cluster)
	if err != nil {
		LogError(ctx, err, "get metastore exposed info failed")
		return nil, err
	}
	tmpKyuubiConf := map[string]string{
		"kyuubi.kubernetes.namespace": cluster.Namespace,
	}
	for k, v := range kyuubi.Configs.Conf {
		tmpKyuubiConf[k] = v
	}
	spark := GetRefClusterInfo(cluster, kyuubi.ClusterRefs[0])
	if spark == nil {
		spark = GetDefaultRefClusterInfo(kyuubi.ClusterRefs[0])
	}

	tmpSparkConf := map[string]string{
		"spark.hadoop.fs.s3a.access.key":             minioExposedInfo.AccessKey,
		"spark.hadoop.fs.s3a.secret.key":             minioExposedInfo.SecretKey,
		"spark.hadoop.fs.s3a.path.style.access":      "true",
		"spark.hadoop.fs.s3a.connection.ssl.enabled": "false",
		"spark.hadoop.fs.s3a.endpoint":               minioFullEndpoint(minioExposedInfo.Endpoint, false),
		"spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.options.claimName":    "OnDemand",
		"spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.options.storageClass": GetStorageClassName(cluster),
		"spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.options.sizeLimit":    DefaultShuffleDiskSize,
		"spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.mount.path":           DefaultShuffleDiskMountPath,
		"spark.kubernetes.executor.volumes.persistentVolumeClaim.spark-local-dir-1.mount.readOnly":       "false",
	}
	for k, v := range spark.Configs.Conf {
		tmpSparkConf[k] = v
	}
	LogInfo(ctx, fmt.Sprintf("sparkConf:%v\n", tmpSparkConf))
	kyuubiDesired := &kov1alpha1.KyuubiCluster{
		ObjectMeta: NineObjectMeta(cluster),
		//Todo,here should be a template instead of hardcoding?
		Spec: kov1alpha1.KyuubiClusterSpec{
			KyuubiVersion: kyuubi.Version,
			KyuubiResource: kov1alpha1.ResourceConfig{
				Replicas: kyuubi.Resource.Replicas,
			},
			KyuubiImage: kov1alpha1.ImageConfig{
				Repository: kyuubi.Configs.Image.Repository,
				Tag:        kyuubi.Configs.Image.Tag,
				PullPolicy: kyuubi.Configs.Image.PullPolicy,
			},
			KyuubiConf: tmpKyuubiConf,
			ClusterRefs: []kov1alpha1.ClusterRef{
				{
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
						SparkDefaults:  tmpSparkConf,
					},
				},
				{
					Name: "metastore",
					Type: "metastore",
					Metastore: kov1alpha1.MetastoreCluster{
						HiveSite: map[string]string{
							"hive.metastore.uris":          "thrift://" + metastoreExposedInfo.ServiceName + "." + cluster.Namespace + ".svc:" + strconv.Itoa(int(metastoreExposedInfo.ServicePort.Port)),
							"hive.metastore.warehouse.dir": "s3a:/" + ninev1alpha1.DataHouseDir,
						},
					},
				},
				{
					Name: "hdfs",
					Type: "hdfs",
					Hdfs: kov1alpha1.HdfsCluster{
						CoreSite: map[string]string{},
						HdfsSite: map[string]string{
							"dfs.client.block.write.retries": "3",
						},
					},
				},
			},
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
