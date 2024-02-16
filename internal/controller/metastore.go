package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	mov1alpha1 "github.com/nineinfra/metastore-operator/api/v1alpha1"
	moversioned "github.com/nineinfra/metastore-operator/client/clientset/versioned"
	moscheme "github.com/nineinfra/metastore-operator/client/clientset/versioned/scheme"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

var (
	MetastoreReplicas int32 = 1
)

func metastoreResourceName(cluster *ninev1alpha1.NineCluster) string {
	return NineResourceName(cluster, MetastoreResourceNameSuffix)
}

func metastoreSvcName(cluster *ninev1alpha1.NineCluster) string {
	return metastoreResourceName(cluster)
}

func constructMetastorePodLabel(name string) map[string]string {
	return map[string]string{
		"cluster": name,
		"app":     DefaultMetastoreClusterSign,
	}
}

func (r *NineClusterReconciler) initMetastore(metastore ninev1alpha1.ClusterInfo) {
	if metastore.Resource.Replicas != 0 {
		MetastoreReplicas = metastore.Resource.Replicas
	}
}

func (r *NineClusterReconciler) constructMetastoreClusterRefs(ctx context.Context, cluster *ninev1alpha1.NineCluster) ([]mov1alpha1.ClusterRef, error) {
	dbc, err := r.getDatabaseExposedInfo(ctx, cluster)
	if err != nil {
		return nil, err
	}

	crs := []mov1alpha1.ClusterRef{
		{
			Name: "database",
			Type: "database",
			Database: mov1alpha1.DatabaseCluster{
				ConnectionUrl: dbc.ConnectionUrl,
				DbType:        dbc.DbType,
				Password:      dbc.Password,
				UserName:      dbc.UserName,
			},
		},
	}

	switch GetClusterStorage(cluster) {
	case ninev1alpha1.NineClusterStorageHdfs:
		hdfsExposedInfo, err := r.getHdfsExposedInfo(ctx, cluster)
		if err != nil {
			return nil, err
		}
		err = r.createHdfsDataDir(ctx, &hdfsExposedInfo, ninev1alpha1.DataHouseDir, 0777)
		if err != nil {
			return nil, err
		}
		crs = append(crs, mov1alpha1.ClusterRef{
			Name: "hdfs",
			Type: "hdfs",
			Hdfs: mov1alpha1.HdfsCluster{
				HdfsSite: r.constructHdfsSite(ctx, cluster),
				CoreSite: r.constructCoreSite(ctx, cluster),
			},
		})
	case ninev1alpha1.NineClusterStorageMinio:
		minioExposedInfo, err := r.getMinioExposedInfo(ctx, cluster)
		if err != nil {
			return nil, err
		}
		err = r.createMinioBucketAndFolder(ctx, &minioExposedInfo, ninev1alpha1.DefaultMinioBucket, ninev1alpha1.DefaultMinioDataHouseFolder, false)
		if err != nil {
			return nil, err
		}
		crs = append(crs, mov1alpha1.ClusterRef{
			Name: "minio",
			Type: "minio",
			Minio: mov1alpha1.MinioCluster{
				Endpoint:        minioFullEndpoint(minioExposedInfo.Endpoint, false),
				AccessKey:       minioExposedInfo.AccessKey,
				SecretKey:       minioExposedInfo.SecretKey,
				SSLEnabled:      "false",
				PathStyleAccess: "true",
			},
		})
	}
	return crs, nil
}

func (r *NineClusterReconciler) constructHiveSite(ctx context.Context, cluster *ninev1alpha1.NineCluster) map[string]string {
	hiveSite := make(map[string]string, 0)
	switch GetClusterStorage(cluster) {
	case ninev1alpha1.NineClusterStorageMinio:
		hiveSite["hive.metastore.warehouse.dir"] = fmt.Sprintf("s3a:/%s", ninev1alpha1.DataHouseDir)
	case ninev1alpha1.NineClusterStorageHdfs:
		hiveSite["hive.metastore.warehouse.dir"] = fmt.Sprintf("hdfs://%s%s", "nineinfra", ninev1alpha1.DataHouseDir)
	}
	return hiveSite
}

func (r *NineClusterReconciler) getMetastoreExposedInfo(ctx context.Context, cluster *ninev1alpha1.NineCluster) (mov1alpha1.ExposedInfo, error) {
	condition := make(chan struct{})
	endpoints := &corev1.Endpoints{}
	go func(ep *corev1.Endpoints) {
		for {
			LogInfoInterval(ctx, 5, "Check metastore pod ready...")
			_, ready := r.CheckPodsReady(cluster, constructMetastorePodLabel(NineResourceName(cluster)), int(MetastoreReplicas))
			if !ready {
				time.Sleep(time.Second)
				continue
			}
			LogInfoInterval(ctx, 5, "Check metastore endpoints ready...")
			_, ready, e := r.CheckEndpointsReady(cluster, metastoreSvcName(cluster), int(MetastoreReplicas))
			if !ready {
				time.Sleep(time.Second)
				continue
			}
			e.DeepCopyInto(ep)
			close(condition)
			break
		}
	}(endpoints)
	<-condition
	LogInfo(ctx, "Get metastore cluster exposed info successfully!")

	me := mov1alpha1.ExposedInfo{}
	me.ServiceName = metastoreSvcName(cluster)
	me.ServicePort.Port = endpoints.Subsets[0].Ports[0].Port
	return me, nil
}

func (r *NineClusterReconciler) constructMetastoreCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, metastore ninev1alpha1.ClusterInfo) (*mov1alpha1.MetastoreCluster, error) {
	clusterRefs, err := r.constructMetastoreClusterRefs(ctx, cluster)
	if err != nil {
		return nil, err
	}

	metastoreConf := r.constructHiveSite(ctx, cluster)
	for k, v := range metastore.Configs.Conf {
		metastoreConf[k] = v
	}
	metastoreDesired := &mov1alpha1.MetastoreCluster{
		ObjectMeta: NineObjectMeta(cluster),
		Spec: mov1alpha1.MetastoreClusterSpec{
			MetastoreVersion: metastore.Version,
			MetastoreResource: mov1alpha1.ResourceConfig{
				Replicas: MetastoreReplicas,
			},
			MetastoreImage: mov1alpha1.ImageConfig{
				Repository: metastore.Configs.Image.Repository,
				Tag:        metastore.Configs.Image.Tag,
				PullPolicy: metastore.Configs.Image.PullPolicy,
			},
			MetastoreConf: metastoreConf,
			ClusterRefs:   clusterRefs,
		},
	}

	if err := ctrl.SetControllerReference(cluster, metastoreDesired, r.Scheme); err != nil {
		return nil, err
	}

	return metastoreDesired, nil
}

func (r *NineClusterReconciler) reconcileMetastoreCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, metastore ninev1alpha1.ClusterInfo, logger logr.Logger) error {
	r.initMetastore(metastore)
	desiredMetastore, err := r.constructMetastoreCluster(ctx, cluster, metastore)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Wait for other resource to construct MetastoreCluster...")
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to construct MetastoreCluster")
		return err
	}

	metav1.AddToGroupVersion(moscheme.Scheme, mov1alpha1.GroupVersion)
	utilruntime.Must(mov1alpha1.AddToScheme(moscheme.Scheme))

	config, err := GetK8sClientConfig()
	if err != nil {
		return err
	}

	mc, err := moversioned.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = mc.MetastoreV1alpha1().MetastoreClusters(cluster.Namespace).Get(context.TODO(), NineResourceName(cluster), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		logger.Info("Start to create a new MetastoreCluster...")
		_, err := mc.MetastoreV1alpha1().MetastoreClusters(cluster.Namespace).Create(context.TODO(), desiredMetastore, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	logger.Info("Reconcile a MetastoreCluster successfully")
	return nil
}
