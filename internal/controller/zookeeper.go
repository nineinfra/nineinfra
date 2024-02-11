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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

const (
	DefaultZKReplicas = 3
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
	endpoints := &corev1.Endpoints{}
	go func(ep *corev1.Endpoints) {
		for {
			LogInfoInterval(ctx, 5, "Check zookeeper ready...")
			_, result, e := r.CheckEndpointsReady(cluster, zkSvcName(cluster), DefaultZKReplicas)
			if !result {
				time.Sleep(time.Second)
				continue
			}
			_, result = r.CheckPodsReady(cluster, constructZookeeperPodLabel(NineResourceName(cluster)), DefaultZKReplicas)
			if !result {
				time.Sleep(time.Second)
				continue
			}
			e.DeepCopyInto(ep)
			close(condition)
			break
		}
	}(endpoints)

	<-condition
	var zkClientPort int32 = 0
	for _, v := range endpoints.Subsets[0].Ports {
		if v.Name == DefaultZKClientPortName {
			zkClientPort = v.Port
		}
	}
	zkIpAndPorts := make([]string, 0)
	for _, ip := range endpoints.Subsets[0].Addresses {
		zkIpAndPorts = append(zkIpAndPorts, fmt.Sprintf("%s:%d", ip.IP, zkClientPort))
	}

	LogInfo(ctx, fmt.Sprintf("Get zookeeper exposed info successfully,zkIpAndPorts:%v", zkIpAndPorts))
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
