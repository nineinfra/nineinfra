package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	clusterscheme "github.com/nineinfra/metastore-operator/client/clientset/versioned/scheme"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	clusterv1 "github.com/nineinfra/zookeeper-operator/api/v1"
	clusterversioned "github.com/nineinfra/zookeeper-operator/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func zkResourceName(cluster *ninev1alpha1.NineCluster) string {
	return NineResourceName(cluster, ZKResourceNameSuffix)
}

func zkSvcName(cluster *ninev1alpha1.NineCluster) string {
	return zkResourceName(cluster)
}

func constructZookeeperPodLabel(name string) map[string]string {
	return map[string]string{
		"cluster": name,
		"app":     DefaultZookeeperClusterSign,
	}
}

func (r *NineClusterReconciler) getZookeeperExposedInfo(ctx context.Context, cluster *ninev1alpha1.NineCluster) ([]string, error) {
	condition := make(chan struct{})
	svc := &corev1.Service{}
	go func(svc *corev1.Service) {
		for {
			LogInfoInterval(ctx, 5, "Try to get zookeeper service...")
			if err := r.Get(ctx, types.NamespacedName{Name: zkSvcName(cluster), Namespace: cluster.Namespace}, svc); err != nil && errors.IsNotFound(err) {
				time.Sleep(time.Second)
				continue
			} else if err != nil {
				LogError(ctx, err, "get zookeeper service failed")
			}
			close(condition)
			break
		}
	}(svc)

	<-condition
	endpoints := &corev1.Endpoints{}
	if err := r.Get(ctx, types.NamespacedName{Name: zkSvcName(cluster), Namespace: cluster.Namespace}, endpoints); err != nil {
		LogError(ctx, err, "get zookeeper endpoints failed")
		return nil, err
	}

	var zkClientPort int32 = 0
	for _, v := range endpoints.Subsets[0].Ports {
		if v.Name == DefaultZKClientPortName {
			zkClientPort = v.Port
		}
	}

	zkPods := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(constructZookeeperPodLabel(NineResourceName(cluster)))
	listOps := &client.ListOptions{
		Namespace:     cluster.Namespace,
		LabelSelector: labelSelector,
	}
	if err := r.Client.List(context.TODO(), zkPods, listOps); err != nil {
		LogError(ctx, err, "get zookeeper pods failed")
		return nil, err
	}
	zkIpAndPorts := make([]string, 0)
	for _, pod := range zkPods.Items {
		zkIpAndPorts = append(zkIpAndPorts, fmt.Sprintf("%s:%d", pod.Status.PodIP, zkClientPort))
	}

	LogInfo(ctx, "Get zookeeper exposed info successfully!")
	return zkIpAndPorts, nil
}

func (r *NineClusterReconciler) constructZookeeperCluster(ctx context.Context, ninecluster *ninev1alpha1.NineCluster, cluster ninev1alpha1.ClusterInfo) (*clusterv1.ZookeeperCluster, error) {
	tmpClusterConf := map[string]string{}
	for k, v := range cluster.Configs.Conf {
		tmpClusterConf[k] = v
	}
	clusterDesired := &clusterv1.ZookeeperCluster{
		ObjectMeta: NineObjectMeta(ninecluster),
		Spec: clusterv1.ZookeeperClusterSpec{
			Version: cluster.Version,
			Resource: clusterv1.ResourceConfig{
				Replicas: 3,
			},
			Image: clusterv1.ImageConfig{
				Repository: cluster.Configs.Image.Repository,
				Tag:        cluster.Configs.Image.Tag,
				PullPolicy: cluster.Configs.Image.PullPolicy,
			},
			Conf: tmpClusterConf,
		},
	}

	if err := ctrl.SetControllerReference(ninecluster, clusterDesired, r.Scheme); err != nil {
		return nil, err
	}

	return clusterDesired, nil
}

func (r *NineClusterReconciler) reconcileZookeeperCluster(ctx context.Context, ninecluster *ninev1alpha1.NineCluster, cluster ninev1alpha1.ClusterInfo, logger logr.Logger) error {
	desiredCluster, err := r.constructZookeeperCluster(ctx, ninecluster, cluster)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Wait for other resource to construct ZookeeperCluster...")
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to construct ZookeeperCluster")
		return err
	}

	metav1.AddToGroupVersion(clusterscheme.Scheme, clusterv1.GroupVersion)
	utilruntime.Must(clusterv1.AddToScheme(clusterscheme.Scheme))

	config, err := GetK8sClientConfig()
	if err != nil {
		return err
	}

	cc, err := clusterversioned.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = cc.ZookeeperV1().ZookeeperClusters(ninecluster.Namespace).Get(context.TODO(), NineResourceName(ninecluster), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		logger.Info("Start to create a new ZookeeperCluster...")
		_, err := cc.ZookeeperV1().ZookeeperClusters(ninecluster.Namespace).Create(context.TODO(), desiredCluster, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	logger.Info("Reconcile a ZookeeperCluster successfully")
	return nil
}
