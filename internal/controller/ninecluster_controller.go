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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"sync"

	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
)

var (
	clusterReconcileLocks = make(map[string]*sync.Mutex)
	locksMutex            sync.Mutex
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
			logger.Error(err, "Error occurred during fetching the object")
		}
		return ctrl.Result{}, err
	}
	requestArray := strings.Split(fmt.Sprint(req), "/")
	requestName := requestArray[1]
	logger.Info(fmt.Sprintf("Reconcile requestName %s,cluster.Name %s", requestName, cluster.Name))
	if requestName == cluster.Name {
		logger.Info("Create or update clusters")
		err = r.reconcileClusters(ctx, &cluster, logger)
		if err != nil {
			logger.Error(err, "Error occurred during create or update clusters")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *NineClusterReconciler) reconcileDatabaseCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, database ninev1alpha1.ClusterInfo, logger logr.Logger) error {
	if database.SubType == "" {
		database.SubType = ninev1alpha1.DefaultDbType
	}
	switch database.SubType {
	case ninev1alpha1.DbTypePostgres:
		err := r.reconcilePGCluster(ctx, cluster, database, logger)
		if err != nil {
			return err
		}
	case ninev1alpha1.DbTypeMysql:
		//Todo
	}

	logger.Info("Reconcile a database successfully")
	return nil
}

func (r *NineClusterReconciler) getUniqueClusterLock(cluster *ninev1alpha1.NineCluster, clus ninev1alpha1.ClusterInfo) *sync.Mutex {
	uniqueName := GenUniqueName4Cluster(cluster, clus.Type)
	locksMutex.Lock()
	defer locksMutex.Unlock()
	if _, ok := clusterReconcileLocks[uniqueName]; !ok {
		clusterReconcileLocks[uniqueName] = &sync.Mutex{}
	}
	return clusterReconcileLocks[uniqueName]
}

func (r *NineClusterReconciler) reconcileRoutine(ctx context.Context, cluster *ninev1alpha1.NineCluster, clus ninev1alpha1.ClusterInfo, logger logr.Logger, mutex *sync.Mutex) {
	go func(clus ninev1alpha1.ClusterInfo, mutex *sync.Mutex) {
		if !mutex.TryLock() {
			logger.Info(fmt.Sprintf("Cluster:%s of %s in %s is under reconciling now", clus.Type, cluster.Name, cluster.Namespace))
			return
		}
		defer mutex.Unlock()
		switch clus.Type {
		case ninev1alpha1.KyuubiClusterType:
			err := r.reconcileKyuubiCluster(ctx, cluster, clus, logger)
			if err != nil {
				logger.Error(err, "Failed to reconcile KyuubiCluster")
			}
		case ninev1alpha1.MetaStoreClusterType:
			err := r.reconcileMetastoreCluster(ctx, cluster, clus, logger)
			if err != nil {
				logger.Error(err, "Failed to reconcile MetastoreCluster")
			}
		case ninev1alpha1.MinioClusterType:
			err := r.reconcileMinioTenant(ctx, cluster, clus, logger)
			if err != nil {
				logger.Error(err, "Failed to reconcile DatabaseCluster")
			}
		case ninev1alpha1.DatabaseClusterType:
			err := r.reconcileDatabaseCluster(ctx, cluster, clus, logger)
			if err != nil {
				logger.Error(err, "Failed to reconcile DatabaseCluster")
			}
		case ninev1alpha1.DorisClusterType:
			err := r.reconcileDorisCluster(ctx, cluster, clus, logger)
			if err != nil {
				logger.Error(err, "Failed to reconcile DorisCluster")
			}
		case ninev1alpha1.ZookeeperClusterType:
			err := r.reconcileZookeeperCluster(ctx, cluster, clus, logger)
			if err != nil {
				logger.Error(err, "Failed to reconcile ZookeeperCluster")
			}
		}
	}(clus, mutex)
}

func (r *NineClusterReconciler) reconcileCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, clus ninev1alpha1.ClusterInfo, logger logr.Logger) {
	clusterLock := r.getUniqueClusterLock(cluster, clus)

	r.reconcileRoutine(ctx, cluster, clus, logger, clusterLock)
}

func (r *NineClusterReconciler) renconcileDataHouse(ctx context.Context, cluster *ninev1alpha1.NineCluster, logger logr.Logger) {
	if err := FillClustersInfo(cluster); err != nil {
		logger.Error(err, "Failed to fill clusters' info")
	}

	//Todo,add check if the cluster running?
	for _, v := range cluster.Spec.ClusterSet {
		r.reconcileCluster(ctx, cluster, v, logger)
	}
}

func (r *NineClusterReconciler) reconcileClusters(ctx context.Context, cluster *ninev1alpha1.NineCluster, logger logr.Logger) error {
	if err := FillNineClusterType(cluster); err != nil {
		return err
	}
	switch cluster.Spec.Type {
	case ninev1alpha1.NineClusterTypeBatch:
		r.renconcileDataHouse(ctx, cluster, logger)
	case ninev1alpha1.NineClusterTypeStream:
		//Todo
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NineClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ninev1alpha1.NineCluster{}).
		Complete(r)
}
