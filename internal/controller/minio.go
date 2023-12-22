package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	dpv1beta1 "github.com/minio/directpv/apis/directpv.min.io/v1beta1"
	miniov7 "github.com/minio/minio-go/v7"
	miniocred "github.com/minio/minio-go/v7/pkg/credentials"
	miniov2 "github.com/minio/operator/apis/minio.min.io/v2"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"time"
)

func tenantStorage(q resource.Quantity) corev1.ResourceList {
	m := make(corev1.ResourceList, 1)
	m[corev1.ResourceStorage] = q
	return m
}

func capacityPerVolume(capacity string, volumes int32) (*resource.Quantity, error) {
	totalQuantity, err := resource.ParseQuantity(capacity)
	if err != nil {
		return nil, err
	}
	return resource.NewQuantity(totalQuantity.Value()/int64(volumes), totalQuantity.Format), nil
}

func minioFullEndpoint(endpoint string, sslEnable bool) string {
	if sslEnable {
		return "https://" + endpoint
	} else {
		return "http://" + endpoint
	}
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
			} else if err != nil {
				LogError(ctx, err, "get minio service failed")
			}
			LogInfoInterval(ctx, 5, "Try to get minio secret...")
			if err := r.Get(ctx, types.NamespacedName{Name: MinioNewUserName(cluster), Namespace: cluster.Namespace}, secret); err != nil && errors.IsNotFound(err) {
				time.Sleep(time.Second)
				continue
			} else if err != nil {
				LogError(ctx, err, "get minio secret failed")
			}
			close(condition)
			break
		}
	}(minioSvc, minioSecret)

	<-condition
	LogInfo(ctx, "Get minio exposed info successfully!")
	me := ninev1alpha1.MinioExposedInfo{}
	me.Endpoint = minioSvc.Spec.ClusterIP
	me.AccessKey = string(minioSecret.Data["CONSOLE_ACCESS_KEY"])
	me.SecretKey = string(minioSecret.Data["CONSOLE_SECRET_KEY"])
	return me, nil
}

func (r *NineClusterReconciler) createMinioBucketAndFolder(ctx context.Context, minioInfo *ninev1alpha1.MinioExposedInfo, bucket string, folder string, sslEnable bool) error {
	mc, err := miniov7.New(minioInfo.Endpoint, &miniov7.Options{
		Creds:  miniocred.NewStaticV4(minioInfo.AccessKey, minioInfo.SecretKey, ""),
		Secure: sslEnable,
	})
	if err != nil {
		return err
	}
	condition := make(chan struct{})
	go func() {
		for {
			LogInfoInterval(ctx, 5, "Try to create minio bucket...")
			exists, err := mc.BucketExists(ctx, bucket)
			if err != nil {
				LogInfo(ctx, err.Error())
				time.Sleep(time.Second)
				continue
			}
			if !exists {
				err = mc.MakeBucket(ctx, bucket, miniov7.MakeBucketOptions{})
				if err != nil {
					LogInfo(ctx, err.Error())
					time.Sleep(time.Second)
					continue
				}
			}
			LogInfo(ctx, "Reconcile minio bucket successfully!")
			close(condition)
			break
		}
	}()
	<-condition
	_, err = mc.PutObject(ctx, bucket, folder, nil, 0, miniov7.PutObjectOptions{})
	if err != nil {
		return err
	}
	LogInfo(ctx, "Reconcile minio bucket and folder successfully!")
	return nil
}

func (r *NineClusterReconciler) reconcileMinioNewUserSecret(ctx context.Context, cluster *ninev1alpha1.NineCluster, minio ninev1alpha1.ClusterInfo) error {
	accessKey, secretKey, err := miniov2.GenerateCredentials()
	secretData := map[string][]byte{
		"CONSOLE_ACCESS_KEY": []byte(accessKey),
		"CONSOLE_SECRET_KEY": []byte(secretKey),
	}
	desiredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MinioNewUserName(cluster),
			Namespace: cluster.Namespace,
			Labels:    NineConstructLabels(cluster),
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
	//Todo, should create root accesskey and secretkey randomly
	strData := fmt.Sprintf("%s%s%s%s%s%s", "export MINIO_ACCESS_KEY=", "TIMJKQV5ZTSITBPK", "\n", "export MINIO_SECRET_KEY=", "5QGECCS3GGE05P2W5RCKVTKOBQ3G4QOX", "\n")
	secretData := map[string][]byte{
		"config.env": []byte(strData),
	}
	desiredSecret := &corev1.Secret{
		ObjectMeta: NineObjectMeta(cluster),
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

func (r *NineClusterReconciler) getDirectPVNodesCount(ctx context.Context) (int32, error) {

	metav1.AddToGroupVersion(dpv1beta1.Scheme, dpv1beta1.SchemeGroupVersion)
	utilruntime.Must(dpv1beta1.AddToScheme(dpv1beta1.Scheme))

	config, err := GetK8sClientConfig()
	if err != nil {
		return 0, err
	}

	dc, err := dpv1beta1.NewForConfig(config)
	if err != nil {
		return 0, err
	}

	dpnodelist, err := dc.DirectPVNodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	dpnodes := int32(len(dpnodelist.Items))
	if dpnodes == 0 {
		return 0, errors.NewServiceUnavailable("directpv node not found")
	}

	return dpnodes, nil
}

func (r *NineClusterReconciler) constructMinioTenant(ctx context.Context, cluster *ninev1alpha1.NineCluster, minio ninev1alpha1.ClusterInfo) (*miniov2.Tenant, error) {
	//Todo, this value should be loaded automatically
	sc := "directpv-min-io"
	tmpBool := false
	q, _ := capacityPerVolume(strconv.Itoa(GiB2Bytes(cluster.Spec.DataVolume)), 4*4)

	if err := r.reconcileMinioTenantConfigSecret(ctx, cluster, minio); err != nil {
		return nil, err
	}

	if err := r.reconcileMinioNewUserSecret(ctx, cluster, minio); err != nil {
		return nil, err
	}

	dpnodes, err := r.getDirectPVNodesCount(ctx)
	if err != nil {
		return nil, err
	}

	mtDesired := &miniov2.Tenant{
		ObjectMeta: NineObjectMeta(cluster),
		Spec: miniov2.TenantSpec{
			Configuration: &corev1.LocalObjectReference{
				Name: NineResourceName(cluster),
			},
			RequestAutoCert: &tmpBool,
			Image:           "minio/minio:" + minio.Version,
			Pools: []miniov2.Pool{
				{
					//Todo,this value should be loaded automatically
					Servers:          dpnodes,
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
					Name: MinioNewUserName(cluster),
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(cluster, mtDesired, r.Scheme); err != nil {
		return nil, err
	}

	return mtDesired, nil
}

func (r *NineClusterReconciler) reconcileMinioTenant(ctx context.Context, cluster *ninev1alpha1.NineCluster, minio ninev1alpha1.ClusterInfo, logger logr.Logger) error {
	desiredMinioTenant, _ := r.constructMinioTenant(ctx, cluster, minio)

	metav1.AddToGroupVersion(miniov2.Scheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(miniov2.AddToScheme(miniov2.Scheme))

	config, err := GetK8sClientConfig()
	if err != nil {
		return err
	}

	mc, err := miniov2.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = mc.Tenants(cluster.Namespace).Get(context.TODO(), NineResourceName(cluster), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		fmt.Println(err, "tenant get failed for:", NineResourceName(cluster))
		return err
	}

	if errors.IsNotFound(err) {
		logger.Info("Start to create a new MinioTenant...")
		_, err := mc.Tenants(cluster.Namespace).Create(context.TODO(), desiredMinioTenant, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	logger.Info("Reconcile a MinioTenant successfully")

	return nil
}
