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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mtv2 "github.com/minio/operator/pkg/apis/minio.min.io/v2"
	kov1alpha1 "github.com/nineinfra/kyuubi-operator/api/v1alpha1"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
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

	var nine ninev1alpha1.NineCluster
	err := r.Get(ctx, req.NamespacedName, &nine)
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

	if requestName == nine.Name {
		logger.Info("Create or update nineclusters")
		err = r.reconcileClusters(ctx, &nine, logger)
		if err != nil {
			logger.Info("Error occurred during create or update nineclusters")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *NineClusterReconciler) reconcileResource(ctx context.Context,
	nine *ninev1alpha1.NineCluster,
	constructFunc func(*ninev1alpha1.NineCluster) (client.Object, error),
	existingResource client.Object,
	resourceName string,
	resourceType string) error {
	logger := log.FromContext(ctx)
	err := r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: nine.Namespace}, existingResource)
	if err != nil && errors.IsNotFound(err) {
		resource, err := constructFunc(nine)
		if err != nil {
			logger.Error(err, fmt.Sprintf("Failed to define new %s resource for Nifi", resourceType))

			// The following implementation will update the status
			meta.SetStatusCondition(&nine.Status.Conditions, metav1.Condition{Type: ninev1alpha1.StateFailed,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create %s for the custom resource (%s): (%s)", resourceType, nine.Name, err)})

			if err := r.Status().Update(ctx, nine); err != nil {
				logger.Error(err, "Failed to update ninecluster status")
				return err
			}

			return err
		}

		logger.Info(fmt.Sprintf("Creating a new %s", resourceType),
			fmt.Sprintf("%s.Namespace", resourceType), resource.GetNamespace(), fmt.Sprintf("%s.Name", resourceType), resource.GetName())

		if err = r.Create(ctx, resource); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to create new %s", resourceType),
				fmt.Sprintf("%s.Namespace", resourceType), resource.GetNamespace(), fmt.Sprintf("%s.Name", resourceType), resource.GetName())
			return err
		}

		if err := r.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: nine.Namespace}, existingResource); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to get newly created %s", resourceType))
			return err
		}

	} else if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to get %s", resourceType))
		return err
	}
	return nil
}

func (r *NineClusterReconciler) constructMinioTenant(nine *ninev1alpha1.NineCluster) (client.Object, error) {
	mtDesired := &mtv2.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nine.Name + "-kyuubi",
			Namespace: nine.Namespace,
			Labels: map[string]string{
				"cluster": nine.Name + "-kyuubi",
				"app":     "kyuubi",
			},
		},
		Spec: mtv2.TenantSpec{},
	}

	if err := ctrl.SetControllerReference(nine, mtDesired, r.Scheme); err != nil {
		return nil, err
	}

	return mtDesired, nil

}

func (r *NineClusterReconciler) reconcileMinioTenant(ctx context.Context, nine *ninev1alpha1.NineCluster, logger logr.Logger) error {

	return nil
}

func (r *NineClusterReconciler) reconcileMetastoreCluster(ctx context.Context, nine *ninev1alpha1.NineCluster, logger logr.Logger) error {
	return nil
}

func (r *NineClusterReconciler) constructKyuubiCluster(nine *ninev1alpha1.NineCluster) (client.Object, error) {
	kyuubiDesired := &kov1alpha1.KyuubiCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nine.Name + "-kyuubi",
			Namespace: nine.Namespace,
			Labels: map[string]string{
				"cluster": nine.Name + "-kyuubi",
				"app":     "kyuubi",
			},
		},
		Spec: kov1alpha1.KyuubiClusterSpec{
			KyuubiVersion: "v1.8.1",
			KyuubiResource: kov1alpha1.ResourceConfig{
				Replicas: 1,
			},
			KyuubiImage: kov1alpha1.ImageConfig{
				Repository: "172.18.123.24/library/kyuubi",
				Tag:        "v1.8.1-minio",
			},
			KyuubiConf: map[string]string{
				"kyuubi.kubernetes.namespace":                 nine.Namespace,
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
							Repository: "172.18.123.24/library/spark",
							Tag:        "v3.2.4-minio",
						},
						SparkNamespace: nine.Namespace,
						SparkDefaults: map[string]string{
							"spark.hadoop.fs.s3a.access.key":             "984GcQyUWobTVl3B",
							"spark.hadoop.fs.s3a.secret.key":             "wE5ffRYxSacalsYT5UAVgo1AMlK2uune",
							"spark.hadoop.fs.s3a.path.style.access":      "true",
							"spark.hadoop.fs.s3a.connection.ssl.enabled": "false",
							"spark.hadoop.fs.s3a.endpoint":               "http://192.168.123.24:31063",
						},
					},
				},
				{
					Name: "metastore",
					Type: "metastore",
					Metastore: kov1alpha1.MetastoreCluster{
						HiveSite: map[string]string{
							"hive.metastore.uris":          "thrift://hive-postgres-s3.dwh.svc:9083",
							"hive.metastore.warehouse.dir": "s3://usr/hive/warehouse",
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

	if err := ctrl.SetControllerReference(nine, kyuubiDesired, r.Scheme); err != nil {
		return nil, err
	}

	return kyuubiDesired, nil
}
func (r *NineClusterReconciler) reconcileKyuubiCluster(ctx context.Context, nine *ninev1alpha1.NineCluster, logger logr.Logger) error {
	kyuubiCluster := &kov1alpha1.KyuubiCluster{}
	kyuubiName := nine.Name + "-kyuubi"
	if err := r.reconcileResource(ctx, nine, r.constructKyuubiCluster, kyuubiCluster, kyuubiName, "KyuubiCluster"); err != nil {
		return err
	}
	return nil
}

func (r *NineClusterReconciler) renconcileDataHouse(ctx context.Context, nine *ninev1alpha1.NineCluster, logger logr.Logger) error {
	//create minio tenant and export minio endpoint,access key and secret key
	err := r.reconcileMinioTenant(ctx, nine, logger)
	if err != nil {
		return err
	}
	//create metastore cluster with minio tenant info
	err = r.reconcileMetastoreCluster(ctx, nine, logger)
	if err != nil {
		return err
	}
	//create kyuubi cluster with minio tenant info
	err = r.reconcileKyuubiCluster(ctx, nine, logger)
	if err != nil {
		return nil
	}

	return nil
}

func (r *NineClusterReconciler) reconcileClusters(ctx context.Context, nine *ninev1alpha1.NineCluster, logger logr.Logger) error {
	switch nine.Spec.Type {
	case ninev1alpha1.DataHouse:
		err := r.renconcileDataHouse(ctx, nine, logger)
		if err != nil {
			return err
		}
	case ninev1alpha1.LakeHouse:
	case ninev1alpha1.DataAndLake:
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NineClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ninev1alpha1.NineCluster{}).
		Complete(r)
}
