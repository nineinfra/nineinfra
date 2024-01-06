package controller

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	dov1 "github.com/selectdb/doris-operator/api/doris/v1"
	doversioned "github.com/selectdb/doris-operator/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	DorisResourceNameSuffix        = "-doris"
	DefaultDorisBeStorageMountPath = "/opt/apache-doris/be/storage"
	DefaultDorisBeStoragePVName    = "bestorage"
	DefaultDorisAdminUser          = "root"
	DefaultDorisAdminPassword      = "root"
)

func (r *NineClusterReconciler) getFEAndBEClusterInfo(cluster *ninev1alpha1.NineCluster, doris ninev1alpha1.ClusterInfo) (*ninev1alpha1.ClusterInfo, *ninev1alpha1.ClusterInfo, error) {
	var fecluster, becluster ninev1alpha1.ClusterInfo
	var fe, be bool
	for _, cType := range doris.ClusterRefs {
		for _, v := range cluster.Spec.ClusterSet {
			if cType == v.Type {
				if cType == ninev1alpha1.DorisFEClusterType {
					v.DeepCopyInto(&fecluster)
					fe = true
				} else if cType == ninev1alpha1.DorisBEClusterType {
					v.DeepCopyInto(&becluster)
					be = true
				}
			}
		}
	}
	if !fe || !be {
		return nil, nil, errors.New("invalid parameters,please supply valid fe and be info")
	}
	return &fecluster, &becluster, nil
}

func (r *NineClusterReconciler) getAdminUserInfo(cluster *ninev1alpha1.NineCluster, doris ninev1alpha1.ClusterInfo) (string, string) {
	userName := doris.Configs.Auth.UserName
	password := doris.Configs.Auth.Password
	if userName == "" {
		userName = DefaultDorisAdminUser
	}
	if password == "" {
		password = DefaultDorisAdminPassword
	}
	return userName, password
}

func (r *NineClusterReconciler) constructDorisCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, doris ninev1alpha1.ClusterInfo) (*dov1.DorisCluster, error) {
	logger := log.FromContext(ctx)
	fecluster, becluster, err := r.getFEAndBEClusterInfo(cluster, doris)
	if err != nil {
		logger.Error(err, "invalid parameters,please supply valid fe and be info!")
		return nil, err
	}
	DorisStorgeClass := GetStorageClassName(&doris)
	replicas := int32(3)
	userName, _ := r.getAdminUserInfo(cluster, doris)
	DorisDesired := &dov1.DorisCluster{
		ObjectMeta: NineObjectMeta(cluster, DorisResourceNameSuffix),
		Spec: dov1.DorisClusterSpec{
			FeSpec: &dov1.FeSpec{
				ElectionNumber: &replicas,
				BaseSpec: dov1.BaseSpec{
					Replicas: &replicas,
					Image:    fecluster.Configs.Image.Repository + ":" + fecluster.Configs.Image.Tag,
				},
			},
			BeSpec: &dov1.BeSpec{
				BaseSpec: dov1.BaseSpec{
					Replicas: &replicas,
					Image:    becluster.Configs.Image.Repository + ":" + becluster.Configs.Image.Tag,
					PersistentVolumes: []dov1.PersistentVolume{
						{
							MountPath: DefaultDorisBeStorageMountPath,
							Name:      DefaultDorisBeStoragePVName,
							PersistentVolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
								AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								StorageClassName: &DorisStorgeClass,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"storage": becluster.Resource.ResourceRequirements.Requests["storage"],
									},
								},
							},
						},
					},
				},
			},
			AdminUser: &dov1.AdminUser{
				Name: userName,
			},
		},
	}

	if err := ctrl.SetControllerReference(cluster, DorisDesired, r.Scheme); err != nil {
		return nil, err
	}

	return DorisDesired, nil
}

func (r *NineClusterReconciler) reconcileDorisCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, doris ninev1alpha1.ClusterInfo, logger logr.Logger) error {
	desiredDorisCluster, _ := r.constructDorisCluster(ctx, cluster, doris)

	metav1.AddToGroupVersion(runtime.NewScheme(), dov1.GroupVersion)
	utilruntime.Must(dov1.AddToScheme(runtime.NewScheme()))

	config, err := GetK8sClientConfig()
	if err != nil {
		return err
	}

	dc, err := doversioned.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = dc.DorisV1().DorisClusters(cluster.Namespace).Get(context.TODO(), NineResourceName(cluster), metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		logger.Error(err, fmt.Sprintf("doris cluster get failed for %s", NineResourceName(cluster)))
		return err
	}

	if k8serrors.IsNotFound(err) {
		logger.Info("Start to create a new DorisCluster...")
		_, err := dc.DorisV1().DorisClusters(cluster.Namespace).Create(context.TODO(), desiredDorisCluster, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	logger.Info("Reconcile a DorisCluster successfully")

	return nil
}
