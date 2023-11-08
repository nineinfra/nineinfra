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

package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kov1alpha1 "github.com/nineinfra/kyuubi-operator/api/v1alpha1"
	koversioned "github.com/nineinfra/kyuubi-operator/client/clientset/versioned"
	koscheme "github.com/nineinfra/kyuubi-operator/client/clientset/versioned/scheme"
	mov1alpha1 "github.com/nineinfra/metastore-operator/api/v1alpha1"
	moversioned "github.com/nineinfra/metastore-operator/client/clientset/versioned"
	moscheme "github.com/nineinfra/metastore-operator/client/clientset/versioned/scheme"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"

	miniov2 "github.com/minio/operator/apis/minio.min.io/v2"
)

// NineClusterReconciler reconciles a NineCluster object
type NineClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=nine.nineinfra.tech,resources=nineclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nine.nineinfra.tech,resources=nineclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nine.nineinfra.tech,resources=nineclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NineCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *NineClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var cluster ninev1alpha1.NineCluster
	err := r.Get(ctx, req.NamespacedName, &cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Object not found, it could have been deleted")
		} else {
			logger.Info("Error occurred during fetching the object")
		}
		return ctrl.Result{}, err
	}
	requestArray := strings.Split(fmt.Sprint(req), "/")
	requestName := requestArray[1]
	strlog := "requestName:" + requestName + " cluster.Name:" + cluster.Name
	logger.Info(strlog)
	if requestName == cluster.Name {
		logger.Info("Create or update clusters")
		err = r.reconcileClusters(ctx, &cluster, logger)
		if err != nil {
			logger.Info("Error occurred during create or update clusters")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *NineClusterReconciler) constructLabels(cluster *ninev1alpha1.NineCluster) map[string]string {
	return map[string]string{
		"cluster": cluster.Name,
		"app":     ninev1alpha1.ClusterSign,
	}
}

func (r *NineClusterReconciler) resourceName(cluster *ninev1alpha1.NineCluster) string {
	return cluster.Name + ninev1alpha1.ClusterNameSuffix
}

func (r *NineClusterReconciler) minioNewUserName(cluster *ninev1alpha1.NineCluster) string {
	return cluster.Name + ninev1alpha1.ClusterNameSuffix + "-user"
}

func (r *NineClusterReconciler) objectMeta(cluster *ninev1alpha1.NineCluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      r.resourceName(cluster),
		Namespace: cluster.Namespace,
		Labels:    r.constructLabels(cluster),
	}
}

func (r *NineClusterReconciler) reconcileResource(ctx context.Context,
	cluster *ninev1alpha1.NineCluster,
	subCluster ninev1alpha1.ClusterInfo,
	constructFunc func(context.Context, *ninev1alpha1.NineCluster, ninev1alpha1.ClusterInfo) (client.Object, error),
	existingResource client.Object,
	resourceName string,
	resourceType string) error {
	logger := log.FromContext(ctx)
	err := r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: cluster.Namespace}, existingResource)
	if err != nil && errors.IsNotFound(err) {
		res, err := constructFunc(ctx, cluster, subCluster)
		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to define new %s resource for Nifi", resourceType))
			// The following implementation will update the status
			meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{Type: ninev1alpha1.StateFailed,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create %s for the custom resource (%s): (%s)", resourceType, cluster.Name, err)})
			if err := r.Status().Update(ctx, cluster); err != nil {
				logger.Error(err, "Failed to update ninecluster status")
				return err
			}
			return err
		}

		logger.Info(fmt.Sprintf("Creating a new %s", resourceType),
			fmt.Sprintf("%s.Namespace", resourceType), res.GetNamespace(), fmt.Sprintf("%s.Name", resourceType), res.GetName())

		if err = r.Create(ctx, res); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to create new %s", resourceType),
				fmt.Sprintf("%s.Namespace", resourceType), res.GetNamespace(), fmt.Sprintf("%s.Name", resourceType), res.GetName())
			return err
		}

		if err := r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: cluster.Namespace}, existingResource); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to get newly created %s", resourceType))
			return err
		}
	} else if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to get %s", resourceType))
		return err
	}
	return nil
}

func tenantStorage(q resource.Quantity) corev1.ResourceList {
	m := make(corev1.ResourceList, 1)
	m[corev1.ResourceStorage] = q
	return m
}

func CapacityPerVolume(capacity string, volumes int32) (*resource.Quantity, error) {
	totalQuantity, err := resource.ParseQuantity(capacity)
	if err != nil {
		return nil, err
	}
	return resource.NewQuantity(totalQuantity.Value()/int64(volumes), totalQuantity.Format), nil
}

func (r *NineClusterReconciler) reconcileMinioNewUserSecret(ctx context.Context, cluster *ninev1alpha1.NineCluster, minio ninev1alpha1.ClusterInfo) error {
	accessKey, secretKey, err := miniov2.GenerateCredentials()
	secretData := map[string][]byte{
		"CONSOLE_ACCESS_KEY": []byte(accessKey),
		"CONSOLE_SECRET_KEY": []byte(secretKey),
	}
	desiredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.minioNewUserName(cluster),
			Namespace: cluster.Namespace,
			Labels:    r.constructLabels(cluster),
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	if err := ctrl.SetControllerReference(cluster, desiredSecret, r.Scheme); err != nil {
		return err
	}

	existingSecret := &corev1.Secret{}

	err = r.Get(ctx, client.ObjectKeyFromObject(desiredSecret), existingSecret)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		if err := r.Create(ctx, desiredSecret); err != nil {
			return err
		}
	}

	return nil
}

func (r *NineClusterReconciler) reconcileMinioTenantConfigSecret(ctx context.Context, cluster *ninev1alpha1.NineCluster, minio ninev1alpha1.ClusterInfo) error {
	//Todo, should get accesskey and secretkey automatically
	strData := fmt.Sprintf("%s%s%s%s%s%s", "export MINIO_ACCESS_KEY=", "TIMJKQV5ZTSITBPK", "\n", "export MINIO_SECRET_KEY=", "5QGECCS3GGE05P2W5RCKVTKOBQ3G4QOX", "\n")
	secretData := map[string][]byte{
		"config.env": []byte(strData),
	}
	desiredSecret := &corev1.Secret{
		ObjectMeta: r.objectMeta(cluster),
		Type:       corev1.SecretTypeOpaque,
		Data:       secretData,
	}

	if err := ctrl.SetControllerReference(cluster, desiredSecret, r.Scheme); err != nil {
		return err
	}

	existingSecret := &corev1.Secret{}

	err := r.Get(ctx, client.ObjectKeyFromObject(desiredSecret), existingSecret)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		if err := r.Create(ctx, desiredSecret); err != nil {
			return err
		}
	}

	return nil
}

func (r *NineClusterReconciler) constructMinioTenant(ctx context.Context, cluster *ninev1alpha1.NineCluster, minio ninev1alpha1.ClusterInfo) (*miniov2.Tenant, error) {
	//Todo, this value should be loaded automatically
	sc := "directpv-min-io"
	tmpBool := false
	q, _ := CapacityPerVolume(strconv.Itoa(GiB2Bytes(cluster.Spec.DataVolume)), 4*4)

	if err := r.reconcileMinioTenantConfigSecret(ctx, cluster, minio); err != nil {
		return nil, err
	}

	if err := r.reconcileMinioNewUserSecret(ctx, cluster, minio); err != nil {
		return nil, err
	}

	mtDesired := &miniov2.Tenant{
		ObjectMeta: r.objectMeta(cluster),
		Spec: miniov2.TenantSpec{
			Configuration: &corev1.LocalObjectReference{
				Name: r.resourceName(cluster),
			},
			RequestAutoCert: &tmpBool,
			Image:           "minio/minio:" + minio.Version,
			Pools: []miniov2.Pool{
				{
					//Todo,this value should be loaded automatically
					Servers:          4,
					VolumesPerServer: 4,
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "data",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.ResourceRequirements{
								Requests: tenantStorage(*q),
							},
							StorageClassName: &sc,
						},
					},
				},
			},
			Users: []*corev1.LocalObjectReference{
				{
					Name: r.minioNewUserName(cluster),
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(cluster, mtDesired, r.Scheme); err != nil {
		return nil, err
	}

	return mtDesired, nil
}

func getK8sClientConfig() (*rest.Config, error) {
	kubeconfigPath := filepath.Join("/etc/kubernetes", "kubeconfig")

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	//config, err := rest.InClusterConfig()
	//if err != nil {
	//	return nil,err
	//}
	return config, nil
}

func (r *NineClusterReconciler) reconcileMinioTenant(ctx context.Context, cluster *ninev1alpha1.NineCluster, minio ninev1alpha1.ClusterInfo, logger logr.Logger) error {
	desiredMinioTenant, _ := r.constructMinioTenant(ctx, cluster, minio)

	metav1.AddToGroupVersion(miniov2.Scheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(miniov2.AddToScheme(miniov2.Scheme))

	config, err := getK8sClientConfig()

	mc, err := miniov2.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = mc.Tenants(cluster.Namespace).Get(context.TODO(), r.resourceName(cluster), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		fmt.Println(err, "tenant get failed for:", r.resourceName(cluster))
		return err
	}

	if errors.IsNotFound(err) {
		logger.Info("Start to create a new MinioTenant...")
		_, err := mc.Tenants(cluster.Namespace).Create(context.TODO(), desiredMinioTenant, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *NineClusterReconciler) getMinioExposedInfo(ctx context.Context, cluster *ninev1alpha1.NineCluster) (ninev1alpha1.MinioExposedInfo, error) {
	condition := make(chan struct{})
	minioSvc := &corev1.Service{}
	minioSecret := &corev1.Secret{}
	go func(svc *corev1.Service, secret *corev1.Secret) {
		for {
			//Todo, dead loop here can be broken manually?
			LogInfoInterval(ctx, 5, "Try to get minio service...")
			if err := r.Get(ctx, types.NamespacedName{Name: "minio", Namespace: cluster.Namespace}, svc); err != nil && errors.IsNotFound(err) {
				time.Sleep(time.Second)
				continue
			}
			LogInfoInterval(ctx, 5, "Try to get minio secret...")
			if err := r.Get(ctx, types.NamespacedName{Name: r.minioNewUserName(cluster), Namespace: cluster.Namespace}, secret); err != nil && errors.IsNotFound(err) {
				time.Sleep(time.Second)
				continue
			}
			close(condition)
			break
		}
	}(minioSvc, minioSecret)

	<-condition
	LogInfo(ctx, "Get minio exposed info successfully!")
	me := ninev1alpha1.MinioExposedInfo{}
	me.Endpoint = "http://" + minioSvc.Spec.ClusterIP
	me.AccessKey = string(minioSecret.Data["CONSOLE_ACCESS_KEY"])
	me.SecretKey = string(minioSecret.Data["CONSOLE_SECRET_KEY"])
	return me, nil
}

func (r *NineClusterReconciler) getMetastoreExposedInfo(ctx context.Context, cluster *ninev1alpha1.NineCluster) (mov1alpha1.ExposedInfo, error) {
	me := mov1alpha1.ExposedInfo{}
	config, err := getK8sClientConfig()

	mclient, err := moversioned.NewForConfig(config)
	if err != nil {
		return me, err
	}
	condition := make(chan struct{})

	mc := &mov1alpha1.MetastoreCluster{}
	go func(metastorecluster *mov1alpha1.MetastoreCluster) {
		for {
			LogInfoInterval(ctx, 5, "Try to get metastore cluster...")
			mctemp, err := mclient.MetastoreV1alpha1().MetastoreClusters(cluster.Namespace).Get(context.TODO(), r.resourceName(cluster), metav1.GetOptions{})
			if err != nil && !errors.IsNotFound(err) {
				time.Sleep(time.Second)
				continue
			}
			LogInfoInterval(ctx, 5, "Try to get metastore cluster status...")
			if mctemp.Status.ExposedInfos == nil {
				time.Sleep(time.Second)
				continue
			}
			mctemp.DeepCopyInto(metastorecluster)
			close(condition)
			break
		}
	}(mc)
	<-condition
	LogInfo(ctx, "Get metastore cluster exposed info successfully!")
	for _, v := range mc.Status.ExposedInfos {
		if v.ExposedType == mov1alpha1.ExposedThriftHttp {
			me = v
			break
		}
	}
	return me, nil
}

func (r *NineClusterReconciler) constructMetastoreCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, metastore ninev1alpha1.ClusterInfo) (*mov1alpha1.MetastoreCluster, error) {
	minioExposedInfo, err := r.getMinioExposedInfo(ctx, cluster)
	if err != nil {
		return nil, err
	}
	metastoreDesired := &mov1alpha1.MetastoreCluster{
		ObjectMeta: r.objectMeta(cluster),
		//Todo,here should be a template instead of hardcoding?
		Spec: mov1alpha1.MetastoreClusterSpec{
			MetastoreVersion: metastore.Version,
			MetastoreResource: mov1alpha1.ResourceConfig{
				Replicas: 1,
			},
			MetastoreImage: mov1alpha1.ImageConfig{
				Repository: "172.18.123.24:30003/library/metastore",
				Tag:        "v3.1.3",
			},
			//Todo,the bucket in minio should be created automatically
			MetastoreConf: map[string]string{
				"hive.metastore.warehouse.dir": "/usr/hive/warehouse",
			},
			ClusterRefs: []mov1alpha1.ClusterRef{
				{
					Name: "database",
					Type: "database",
					Database: mov1alpha1.DatabaseCluster{
						ConnectionUrl: "jdbc:postgresql://postgresql:5432/hive",
						DbType:        "postgres",
						Password:      "hive",
						UserName:      "hive",
					},
				},
				{
					Name: "minio",
					Type: "minio",
					Minio: mov1alpha1.MinioCluster{
						Endpoint:        minioExposedInfo.Endpoint,
						AccessKey:       minioExposedInfo.AccessKey,
						SecretKey:       minioExposedInfo.SecretKey,
						SSLEnabled:      "false",
						PathStyleAccess: "true",
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(cluster, metastoreDesired, r.Scheme); err != nil {
		return nil, err
	}

	return metastoreDesired, nil
}

func (r *NineClusterReconciler) reconcileMetastoreCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, metastore ninev1alpha1.ClusterInfo, logger logr.Logger) error {
	desiredMetastore, err := r.constructMetastoreCluster(ctx, cluster, metastore)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Wait for other resource to constructMetastoreCluster...")
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to constructMetastoreCluster")
		return err
	}

	metav1.AddToGroupVersion(moscheme.Scheme, mov1alpha1.GroupVersion)
	utilruntime.Must(mov1alpha1.AddToScheme(moscheme.Scheme))

	config, err := getK8sClientConfig()

	mc, err := moversioned.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = mc.MetastoreV1alpha1().MetastoreClusters(cluster.Namespace).Get(context.TODO(), r.resourceName(cluster), metav1.GetOptions{})
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

	return nil
}

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
		ObjectMeta: r.objectMeta(cluster),
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
							"spark.hadoop.fs.s3a.endpoint":               minioExposedInfo.Endpoint,
						},
					},
				},
				{
					Name: "metastore",
					Type: "metastore",
					Metastore: kov1alpha1.MetastoreCluster{
						HiveSite: map[string]string{
							"hive.metastore.uris":          "thrift://" + metastoreExposedInfo.ServiceName + "." + cluster.Namespace + ".svc:" + strconv.Itoa(int(metastoreExposedInfo.ServicePort.Port)),
							"hive.metastore.warehouse.dir": "s3a://usr/hive/warehouse",
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
		logger.Info("Wait for other resource to constructKyuubiCluster...")
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to constructKyuubiCluster")
		return err
	}

	metav1.AddToGroupVersion(koscheme.Scheme, kov1alpha1.GroupVersion)
	utilruntime.Must(kov1alpha1.AddToScheme(koscheme.Scheme))

	config, err := getK8sClientConfig()

	kc, err := koversioned.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = kc.KyuubiV1alpha1().KyuubiClusters(cluster.Namespace).Get(context.TODO(), r.resourceName(cluster), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		logger.Info("Start to create a new KyuubiCluster...")
		_, err := kc.KyuubiV1alpha1().KyuubiClusters(cluster.Namespace).Create(context.TODO(), desiredKyuubi, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *NineClusterReconciler) renconcileDataHouse(ctx context.Context, cluster *ninev1alpha1.NineCluster, logger logr.Logger) {
	if cluster.Spec.ClusterSet == nil {
		cluster.Spec.ClusterSet = ninev1alpha1.NineDatahouseClusterset
	}
	for _, v := range cluster.Spec.ClusterSet {
		switch v.Type {
		case ninev1alpha1.KyuubiClusterType:
			//create kyuubi cluster with minio tenant info
			go func(clus ninev1alpha1.ClusterInfo) {
				err := r.reconcileKyuubiCluster(ctx, cluster, clus, logger)
				if err != nil {
					logger.Error(err, "Failed to reconcileKyuubiCluster")
				}
			}(v)
		case ninev1alpha1.MetaStoreClusterType:
			//create metastore cluster with minio tenant info
			go func(clus ninev1alpha1.ClusterInfo) {
				err := r.reconcileMetastoreCluster(ctx, cluster, clus, logger)
				if err != nil {
					logger.Error(err, "Failed to reconcileMetastoreCluster")
				}
			}(v)
		case ninev1alpha1.MinioClusterType:
			//create minio tenant and export minio endpoint,access key and secret key
			go func(clus ninev1alpha1.ClusterInfo) {
				err := r.reconcileMinioTenant(ctx, cluster, clus, logger)
				if err != nil {
					logger.Error(err, "Failed to reconcileMinioTenant")
				}
			}(v)
		}
	}
}

func (r *NineClusterReconciler) reconcileClusters(ctx context.Context, cluster *ninev1alpha1.NineCluster, logger logr.Logger) error {
	if cluster.Spec.Type == "" {
		cluster.Spec.Type = ninev1alpha1.DataHouse
	}
	switch cluster.Spec.Type {
	case ninev1alpha1.DataHouse:
		r.renconcileDataHouse(ctx, cluster, logger)
	case ninev1alpha1.DataLake:
		//todo
	case ninev1alpha1.HouseAndLake:
		//todo
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NineClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ninev1alpha1.NineCluster{}).
		Complete(r)
}
