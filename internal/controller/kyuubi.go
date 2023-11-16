package controller

import (
	"context"
	"github.com/go-logr/logr"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"strconv"

	kov1alpha1 "github.com/nineinfra/kyuubi-operator/api/v1alpha1"
	koversioned "github.com/nineinfra/kyuubi-operator/client/clientset/versioned"
	koscheme "github.com/nineinfra/kyuubi-operator/client/clientset/versioned/scheme"
)

func (r *NineClusterReconciler) constructKyuubiCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, kyuubi ninev1alpha1.ClusterInfo) (*kov1alpha1.KyuubiCluster, error) {
	minioExposedInfo, err := r.getMinioExposedInfo(ctx, cluster)
	if err != nil {
		return nil, err
	}
	metastoreExposedInfo, err := r.getMetastoreExposedInfo(ctx, cluster)
	if err != nil {
		return nil, err
	}
	kyuubiDesired := &kov1alpha1.KyuubiCluster{
		ObjectMeta: NineObjectMeta(cluster),
		//Todo,here should be a template instead of hardcoding?
		Spec: kov1alpha1.KyuubiClusterSpec{
			KyuubiVersion: kyuubi.Version,
			KyuubiResource: kov1alpha1.ResourceConfig{
				Replicas: 1,
			},
			KyuubiImage: kov1alpha1.ImageConfig{
				Repository: "172.18.123.24:30003/library/kyuubi",
				Tag:        "v1.8.1-minio",
			},
			KyuubiConf: map[string]string{
				"kyuubi.kubernetes.namespace":                 cluster.Namespace,
				"kyuubi.frontend.connection.url.use.hostname": "false",
				"kyuubi.frontend.thrift.binary.bind.port":     "10009",
				"kyuubi.frontend.thrift.http.bind.port":       "10010",
				"kyuubi.frontend.rest.bind.port":              "10099",
				"kyuubi.frontend.mysql.bind.port":             "3309",
				"kyuubi.frontend.protocols":                   "REST,THRIFT_BINARY",
				"kyuubi.metrics.enabled":                      "false",
			},
			ClusterRefs: []kov1alpha1.ClusterRef{
				{
					Name: "spark",
					Type: "spark",
					Spark: kov1alpha1.SparkCluster{
						SparkMaster: "k8s",
						SparkImage: kov1alpha1.ImageConfig{
							Repository: "172.18.123.24:30003/library/spark",
							Tag:        "v3.2.4-minio",
						},
						SparkNamespace: cluster.Namespace,
						SparkDefaults: map[string]string{
							"spark.hadoop.fs.s3a.access.key":             minioExposedInfo.AccessKey,
							"spark.hadoop.fs.s3a.secret.key":             minioExposedInfo.SecretKey,
							"spark.hadoop.fs.s3a.path.style.access":      "true",
							"spark.hadoop.fs.s3a.connection.ssl.enabled": "false",
							"spark.hadoop.fs.s3a.endpoint":               minioFullEndpoint(minioExposedInfo.Endpoint, false),
						},
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
