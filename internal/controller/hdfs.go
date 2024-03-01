package controller

import (
	"context"
	"fmt"
	hdfsv2 "github.com/colinmarc/hdfs/v2"
	"github.com/go-logr/logr"
	clusterv1 "github.com/nineinfra/hdfs-operator/api/v1"
	clusterversioned "github.com/nineinfra/hdfs-operator/client/clientset/versioned"
	clusterscheme "github.com/nineinfra/metastore-operator/client/clientset/versioned/scheme"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	githuberrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DfsReplicationConfKey = "dfs.replication"
	FSDefaultFSConfKey    = "fs.defaultFS"
	DFSNameSpacesConfKey  = "dfs.nameservices"
)

var (
	DefaultDfsReplication   = 3
	DefaultDataNodeReplicas = 3
	// DefaultNameService is the default name service for hdfs
	DefaultNameService     = "nineinfra"
	DefaultNameNodeRpcPort = 8020
	// DefaultNameNodeHaReplicas is the default ha replicas for namenode
	DefaultNameNodeHaReplicas = 2
)

func getDfsReplication(conf map[string]string) int {
	if value, ok := conf[DfsReplicationConfKey]; ok {
		r, err := strconv.ParseInt(value, 0, 0)
		if err == nil {
			return int(r)
		}
	}
	return DefaultDfsReplication
}

func checkHdfsNameNodeHA(cluster *ninev1alpha1.NineCluster) bool {
	for _, c := range cluster.Spec.ClusterSet {
		if c.Type == ninev1alpha1.HdfsNameNodeClusterType {
			if c.Resource.Replicas == 2 {
				return true
			}
		}
	}
	return false
}

func (r *NineClusterReconciler) createHdfsDataDir(ctx context.Context, hdfsInfo *ninev1alpha1.HdfsExposedInfo, dir string, mode os.FileMode) error {
	hcOptions := hdfsv2.ClientOptions{
		User: "root",
	}
	hcOptions.Addresses = make([]string, 0)
	if hdfsInfo.HdfsSite != nil {
		if _, ok := hdfsInfo.HdfsSite[DFSNameSpacesConfKey]; ok {
			if _, ok := hdfsInfo.HdfsSite[fmt.Sprintf("dfs.ha.namenodes.%s", hdfsInfo.HdfsSite[DFSNameSpacesConfKey])]; ok {
				nameNodes := hdfsInfo.HdfsSite[fmt.Sprintf("dfs.ha.namenodes.%s", hdfsInfo.HdfsSite[DFSNameSpacesConfKey])]
				nameNodeList := strings.Split(nameNodes, ",")
				for _, v := range nameNodeList {
					hcOptions.Addresses = append(hcOptions.Addresses, hdfsInfo.HdfsSite[fmt.Sprintf("dfs.namenode.rpc-address.%s.%s", hdfsInfo.HdfsSite[DFSNameSpacesConfKey], v)])
				}
			} else {
				hcOptions.Addresses = append(hcOptions.Addresses, hdfsInfo.HdfsSite[fmt.Sprintf("dfs.namenode.rpc-address.%s", hdfsInfo.HdfsSite[DFSNameSpacesConfKey])])
			}
		}
	}
	hc, err := hdfsv2.NewClient(hcOptions)
	if err != nil {
		return err
	}
	condition := make(chan struct{})
	go func() {
		for {
			LogInfoInterval(ctx, 5, fmt.Sprintf("Check hdfs directory %s exists...", dir))
			stat, err := hc.Stat(dir)
			if err != nil && !strings.Contains(err.Error(), os.ErrNotExist.Error()) {
				LogInfo(ctx, fmt.Sprintf("Stat directory:%s,err:%s", dir, err.Error()))
				time.Sleep(time.Second)
				continue
			}
			if err != nil && strings.Contains(err.Error(), os.ErrNotExist.Error()) {
				LogInfoInterval(ctx, 5, fmt.Sprintf("Try to create hdfs directory %s...", dir))
				err = hc.MkdirAll(dir, mode)
				if err != nil {
					LogInfo(ctx, fmt.Sprintf("MkdirAll dir:%s,err:%s", dir, err.Error()))
					time.Sleep(time.Second)
					continue
				}
			}
			if stat != nil && !stat.IsDir() {
				LogError(ctx, githuberrors.New("file already exists"), fmt.Sprintf("A file with a same name with the directory %s exist,you should move the file first", dir))
				time.Sleep(5 * time.Second)
				continue
			}
			LogInfo(ctx, "Reconcile hdfs directory successfully!")
			close(condition)
			break
		}
	}()
	<-condition

	return nil
}

func (r *NineClusterReconciler) getHdfsExposedInfo(ctx context.Context, cluster *ninev1alpha1.NineCluster) (ninev1alpha1.HdfsExposedInfo, error) {
	condition := make(chan struct{})
	replicas := 0
	for _, c := range cluster.Spec.ClusterSet {
		if c.Type == ninev1alpha1.HdfsNameNodeClusterType {
			replicas = int(c.Resource.Replicas)
		}
	}
	var waitErr error = nil
	go func(clus *ninev1alpha1.NineCluster, waitErr *error) {
		waitSeconds := 0
		for {
			LogInfoInterval(ctx, 5, "Try to get name node endpoints...")
			nnEndpoints := &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NineResourceName(clus, HdfsResourceNameSuffix, HdfsRoleNameNode),
					Namespace: clus.Namespace,
				},
			}
			existsEndpoints := &corev1.Endpoints{}
			err := r.Get(context.TODO(), types.NamespacedName{Name: nnEndpoints.Name, Namespace: nnEndpoints.Namespace}, existsEndpoints)
			if err != nil && errors.IsNotFound(err) {
				waitSeconds += 1
				if waitSeconds < DefaultMaxWaitSeconds {
					time.Sleep(time.Second)
					continue
				} else {
					LogError(ctx, err, "wait for journal node service timeout")
					*waitErr = githuberrors.New("wait for journal node service timeout")
					close(condition)
					break
				}
			} else if err != nil {
				LogError(ctx, err, "get journal node endpoints failed")
			}
			needReplicas := replicas
			LogInfoInterval(ctx, 5, fmt.Sprintf("subsets:%v,needReplicas:%d\n", existsEndpoints.Subsets, needReplicas))
			if len(existsEndpoints.Subsets) > 0 {
				LogInfo(ctx, fmt.Sprintf("addresses in subset 0:%v and len:%d\n",
					existsEndpoints.Subsets[0].Addresses,
					len(existsEndpoints.Subsets[0].Addresses)))
			}
			if len(existsEndpoints.Subsets) == 0 ||
				(len(existsEndpoints.Subsets) > 0 &&
					len(existsEndpoints.Subsets[0].Addresses) < int(needReplicas)) {
				time.Sleep(time.Second)
				continue
			}
			close(condition)
			break
		}
	}(cluster, &waitErr)
	<-condition

	hdfsSite := r.constructHdfsSite(ctx, cluster)
	coreSite := r.constructCoreSite(ctx, cluster)
	return ninev1alpha1.HdfsExposedInfo{
		DefaultFS: coreSite[FSDefaultFSConfKey],
		HdfsSite:  hdfsSite,
		CoreSite:  coreSite,
	}, nil
}

func (r *NineClusterReconciler) getHdfsClusterInfo(ctx context.Context, cluster *ninev1alpha1.NineCluster) *ninev1alpha1.ClusterInfo {
	if cluster.Spec.ClusterSet != nil {
		for _, v := range cluster.Spec.ClusterSet {
			if v.Type == ninev1alpha1.HdfsClusterType {
				return &v
			}
		}
	}
	return nil
}

func (r *NineClusterReconciler) constructCoreSite(ctx context.Context, cluster *ninev1alpha1.NineCluster) map[string]string {
	coreSite := make(map[string]string, 0)
	c := r.getHdfsClusterInfo(ctx, cluster)
	if c != nil && c.Configs.Conf != nil {
		if value, ok := c.Configs.Conf[FSDefaultFSConfKey]; ok {
			coreSite[FSDefaultFSConfKey] = value
			return coreSite
		}
	}
	if checkHdfsNameNodeHA(cluster) {
		coreSite[FSDefaultFSConfKey] = fmt.Sprintf("hdfs://%s", DefaultNameService)
	} else {
		coreSite[FSDefaultFSConfKey] = fmt.Sprintf(
			"hdfs://%s-namenode-%d.%s-namenode.%s.svc.%s:%d",
			NineResourceName(cluster, HdfsResourceNameSuffix),
			0,
			NineResourceName(cluster, HdfsResourceNameSuffix),
			cluster.Namespace,
			GetClusterDomain(cluster, ninev1alpha1.HdfsClusterType),
			DefaultNameNodeRpcPort)
	}

	return coreSite
}

func (r *NineClusterReconciler) constructHdfsSite(ctx context.Context, cluster *ninev1alpha1.NineCluster) map[string]string {
	hdfsSite := make(map[string]string, 0)
	c := r.getHdfsClusterInfo(ctx, cluster)
	if c != nil && c.Configs.Conf != nil {
		if value, ok := c.Configs.Conf[DFSNameSpacesConfKey]; ok {
			hdfsSite[DFSNameSpacesConfKey] = value
		}
	}

	if _, ok := hdfsSite[DFSNameSpacesConfKey]; !ok {
		hdfsSite[DFSNameSpacesConfKey] = DefaultNameService
	}

	if checkHdfsNameNodeHA(cluster) {
		if _, ok := hdfsSite[fmt.Sprintf("dfs.ha.namenodes.%s", hdfsSite["dfs.nameservices"])]; !ok {
			hdfsSite[fmt.Sprintf("dfs.ha.namenodes.%s", hdfsSite["dfs.nameservices"])] = "nn0,nn1"
			for i := 0; i < DefaultNameNodeHaReplicas; i++ {
				hdfsSite[fmt.Sprintf("dfs.namenode.rpc-address.%s.nn%d", hdfsSite["dfs.nameservices"], i)] = fmt.Sprintf(
					"%s-namenode-%d.%s-namenode.%s.svc.%s:%d",
					NineResourceName(cluster, HdfsResourceNameSuffix),
					i,
					NineResourceName(cluster, HdfsResourceNameSuffix),
					cluster.Namespace,
					GetClusterDomain(cluster, ninev1alpha1.HdfsClusterType),
					DefaultNameNodeRpcPort)
				hdfsSite[fmt.Sprintf("dfs.namenode.http-address.%s.nn%d", hdfsSite["dfs.nameservices"], i)] = fmt.Sprintf(
					"%s-namenode-%d.%s-namenode.%s.svc.%s:%d",
					NineResourceName(cluster, HdfsResourceNameSuffix),
					i,
					NineResourceName(cluster, HdfsResourceNameSuffix),
					cluster.Namespace,
					GetClusterDomain(cluster, ninev1alpha1.HdfsClusterType),
					DefaultNameNodeRpcPort)
			}
			hdfsSite[fmt.Sprintf("dfs.client.failover.proxy.provider.%s", hdfsSite["dfs.nameservices"])] = "org.apache.hadoop.hdfs.server.namenode.ha.ConfiguredFailoverProxyProvider"
		}
	} else {
		if _, ok := hdfsSite[fmt.Sprintf("dfs.namenode.rpc-address.%s", hdfsSite["dfs.nameservices"])]; !ok {
			hdfsSite[fmt.Sprintf("dfs.namenode.rpc-address.%s", hdfsSite["dfs.nameservices"])] = fmt.Sprintf(
				"%s-namenode-%d.%s-namenode.%s.svc.%s:%d",
				NineResourceName(cluster, HdfsResourceNameSuffix),
				0,
				NineResourceName(cluster, HdfsResourceNameSuffix),
				cluster.Namespace,
				GetClusterDomain(cluster, ninev1alpha1.HdfsClusterType),
				DefaultNameNodeRpcPort)
		}
	}

	return hdfsSite
}

func (r *NineClusterReconciler) constructHdfsCluster(ctx context.Context, ninecluster *ninev1alpha1.NineCluster, cluster ninev1alpha1.ClusterInfo, logger logr.Logger) (*clusterv1.HdfsCluster, error) {
	clusterConf := map[string]string{}
	for k, v := range cluster.Configs.Conf {
		clusterConf[k] = v
	}
	dfsReplication := getDfsReplication(clusterConf)
	k8sConf := map[string]string{}
	for k, v := range cluster.Configs.Conf {
		k8sConf[k] = v
	}
	namenodeHA := false
	clusters := make([]clusterv1.Cluster, 0)
	for _, ct := range cluster.ClusterRefs {
		for _, c := range ninecluster.Spec.ClusterSet {
			if c.Type == ct {
				tempCluster := clusterv1.Cluster{
					Version: c.Version,
					Type:    clusterv1.ClusterType(c.Type),
					Name:    c.Name,
					Image: clusterv1.ImageConfig{
						Repository:  c.Configs.Image.Repository,
						Tag:         c.Configs.Image.Tag,
						PullPolicy:  c.Configs.Image.PullPolicy,
						PullSecrets: c.Configs.Image.PullSecrets,
					},
					Resource: clusterv1.ResourceConfig{
						Replicas:             c.Resource.Replicas,
						ResourceRequirements: c.Resource.ResourceRequirements,
						StorageClass:         c.Resource.StorageClass,
						Disks:                c.Resource.Disks,
					},
					Conf: c.Configs.Conf,
				}
				if c.Type == ninev1alpha1.HdfsDataNodeClusterType {
					if c.Resource.Replicas == 0 {
						dpns, err := r.getDirectPVNodesCount(ctx)
						if err != nil {
							tempCluster.Resource.Replicas = int32(DefaultDataNodeReplicas)
						} else {
							tempCluster.Resource.Replicas = dpns
						}
					} else {
						tempCluster.Resource.Replicas = c.Resource.Replicas
					}
					q, _ := CapacityPerVolume(strconv.Itoa(GiB2Bytes(ninecluster.Spec.DataVolume*dfsReplication)), tempCluster.Resource.Replicas*int32(GetDiskNum(cluster)))
					logger.Info(fmt.Sprintf("Calc request storage,datavolume:%d,dfsReplication:%d,disknum:%d,replicas:%d", ninecluster.Spec.DataVolume, dfsReplication, GetDiskNum(cluster), tempCluster.Resource.Replicas))
					tempCluster.Resource.ResourceRequirements = corev1.ResourceRequirements{
						Requests: RequestStorage(*q),
					}
					tempCluster.Resource.Disks = int32(GetDiskNum(cluster))
				}
				clusters = append(clusters, tempCluster)
			}
			if c.Type == ninev1alpha1.HdfsNameNodeClusterType && c.Resource.Replicas == 2 {
				namenodeHA = true
			}

		}
	}
	zkCluster := clusterv1.Cluster{}
	if namenodeHA {
		zkIpAndPorts, err := r.getZookeeperExposedInfo(ctx, ninecluster)
		if err != nil {
			return nil, err
		}
		for _, c := range ninecluster.Spec.ClusterSet {
			if c.Type == ninev1alpha1.ZookeeperClusterType {
				zkCluster = clusterv1.Cluster{
					Version: c.Version,
					Name:    c.Name,
					Type:    clusterv1.ClusterType(c.Type),
					Image: clusterv1.ImageConfig{
						Repository:  c.Configs.Image.Repository,
						Tag:         c.Configs.Image.Tag,
						PullPolicy:  c.Configs.Image.PullPolicy,
						PullSecrets: c.Configs.Image.PullSecrets,
					},
					Resource: clusterv1.ResourceConfig{
						Replicas:             c.Resource.Replicas,
						ResourceRequirements: c.Resource.ResourceRequirements,
						StorageClass:         c.Resource.StorageClass,
						Disks:                c.Resource.Disks,
					},
					Conf: c.Configs.Conf,
				}
				if zkCluster.Conf == nil {
					zkCluster.Conf = make(map[string]string, 0)
				}

				zkCluster.Conf[clusterv1.RefClusterZKReplicasKey] = strconv.Itoa(len(zkIpAndPorts))
				zkCluster.Conf[clusterv1.RefClusterZKEndpointsKey] = strings.Join(zkIpAndPorts, ",")
			}
		}
	}

	clusterDesired := &clusterv1.HdfsCluster{
		ObjectMeta: NineObjectMeta(ninecluster),
		Spec: clusterv1.HdfsClusterSpec{
			Version: cluster.Version,
			Resource: clusterv1.ResourceConfig{
				Replicas:             cluster.Resource.Replicas,
				ResourceRequirements: cluster.Resource.ResourceRequirements,
				StorageClass:         cluster.Resource.StorageClass,
				Disks:                cluster.Resource.Disks,
			},
			Image: clusterv1.ImageConfig{
				Repository:  cluster.Configs.Image.Repository,
				Tag:         cluster.Configs.Image.Tag,
				PullPolicy:  cluster.Configs.Image.PullPolicy,
				PullSecrets: cluster.Configs.Image.PullSecrets,
			},
			Conf:     clusterConf,
			K8sConf:  k8sConf,
			Clusters: clusters,
			ClusterRefs: []clusterv1.Cluster{
				zkCluster,
			},
		},
	}

	if err := ctrl.SetControllerReference(ninecluster, clusterDesired, r.Scheme); err != nil {
		return nil, err
	}

	return clusterDesired, nil
}

func (r *NineClusterReconciler) reconcileHdfsCluster(ctx context.Context, ninecluster *ninev1alpha1.NineCluster, cluster ninev1alpha1.ClusterInfo, logger logr.Logger) error {
	desiredCluster, err := r.constructHdfsCluster(ctx, ninecluster, cluster, logger)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Wait for other resource to construct HdfsCluster...")
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to construct HdfsCluster")
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

	_, err = cc.HdfsV1().HdfsClusters(ninecluster.Namespace).Get(context.TODO(), NineResourceName(ninecluster), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		logger.Info("Start to create a new HdfsCluster...")
		_, err := cc.HdfsV1().HdfsClusters(ninecluster.Namespace).Create(context.TODO(), desiredCluster, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	logger.Info("Reconcile a HdfsCluster successfully")
	return nil
}
