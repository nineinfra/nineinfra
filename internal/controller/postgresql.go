package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"net/url"
	"time"

	poversioned "github.com/cloudnative-pg/client/clientset/versioned"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	ninev1alpha1 "github.com/nineinfra/nineinfra/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PGInitDBName     = "hive"
	PGInitDBUserName = "hive"
	PGInitDBPassword = "hive"
)

func getPGSuperUserNameAndPassword(pg ninev1alpha1.ClusterInfo) (string, string) {
	if pg.Configs.Auth.AuthType == ninev1alpha1.ClusterAuthTypeSimple {
		if pg.Configs.Auth.UserName != "" && pg.Configs.Auth.Password != "" {
			return pg.Configs.Auth.UserName, pg.Configs.Auth.Password
		}
	}
	return DefaultPGSuperUserName, DefaultPGSuperUserPassword
}

func pgResourceName(cluster *ninev1alpha1.NineCluster) string {
	return NineResourceName(cluster, PGResourceNameSuffix)
}

func pgRWSvcName(cluster *ninev1alpha1.NineCluster) string {
	return pgResourceName(cluster) + "-rw"
}

func pgJDBCConnetionURL(ctx context.Context, cluster *ninev1alpha1.NineCluster) string {
	return "jdbc:postgresql://" + pgRWSvcName(cluster) + ":5432/" + PGInitDBName
}

func buildPGUri(username string, password string, host string, dbname string) string {
	postgresURI := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(username, password),
		Host:   host,
		Path:   dbname,
	}

	return postgresURI.String()
}

func buildPGJdbc(username string, password string, host string, dbname string) string {
	jdbcURI := &url.URL{
		Scheme: "jdbc:postgresql",
		Host:   host,
		Path:   dbname,
	}
	q := jdbcURI.Query()
	q.Set("user", username)
	q.Set("password", password)
	jdbcURI.RawQuery = q.Encode()
	return jdbcURI.String()
}

func (r *NineClusterReconciler) getDatabaseExposedInfo(ctx context.Context, cluster *ninev1alpha1.NineCluster) (ninev1alpha1.DatabaseCluster, error) {
	condition := make(chan struct{})
	dbSvc := &corev1.Service{}
	go func(svc *corev1.Service) {
		for {
			//Todo, dead loop here can be broken manually?
			LogInfoInterval(ctx, 5, "Try to get db service...")
			if err := r.Get(ctx, types.NamespacedName{Name: pgRWSvcName(cluster), Namespace: cluster.Namespace}, svc); err != nil && errors.IsNotFound(err) {
				time.Sleep(time.Second)
				continue
			}
			close(condition)
			break
		}
	}(dbSvc)

	<-condition
	LogInfo(ctx, "Get database exposed info successfully!")
	dbc := ninev1alpha1.DatabaseCluster{}
	dbc.DbType = ninev1alpha1.DbTypePostgres
	dbc.UserName = PGInitDBUserName
	dbc.Password = PGInitDBPassword
	dbc.ConnectionUrl = pgJDBCConnetionURL(ctx, cluster)
	return dbc, nil
}

func (r *NineClusterReconciler) reconcilePGInitDBUserSecret(ctx context.Context, cluster *ninev1alpha1.NineCluster, database ninev1alpha1.ClusterInfo) error {
	secretData := map[string][]byte{
		"username": []byte(PGInitDBUserName),
		"password": []byte(PGInitDBPassword),
	}
	desiredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PGInitDBUserSecretName(cluster),
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

func (r *NineClusterReconciler) reconcilePGSuperUserSecret(ctx context.Context, cluster *ninev1alpha1.NineCluster, database ninev1alpha1.ClusterInfo) error {
	superUserName, superUserPassword := getPGSuperUserNameAndPassword(database)
	dbname := "*"
	secretData := map[string]string{
		"username": superUserName,
		"password": superUserPassword,
		"user":     superUserName,
		"dbname":   dbname,
		"host":     pgRWSvcName(cluster),
		"port":     fmt.Sprintf("%d", DefaultPGServerPort),
		"pgpass": fmt.Sprintf(
			"%v:%v:%v:%v:%v\n",
			pgRWSvcName(cluster),
			DefaultPGServerPort,
			dbname,
			superUserName,
			superUserPassword),
		"uri":      buildPGUri(superUserName, superUserPassword, pgRWSvcName(cluster), dbname),
		"jdbc-uri": buildPGJdbc(superUserName, superUserPassword, pgRWSvcName(cluster), dbname),
	}
	desiredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PGSuperUserSecretName(cluster),
			Namespace: cluster.Namespace,
			Labels:    NineConstructLabels(cluster),
		},
		Type:       corev1.SecretTypeBasicAuth,
		StringData: secretData,
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

func (r *NineClusterReconciler) constructPGCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, pg ninev1alpha1.ClusterInfo) (*cnpgv1.Cluster, error) {
	PGStorgeClass := GetStorageClassName(&pg)
	enableSupseruserAccess := true
	PGDesired := &cnpgv1.Cluster{
		ObjectMeta: NineObjectMeta(cluster, PGResourceNameSuffix),
		Spec: cnpgv1.ClusterSpec{
			Instances: 3,
			StorageConfiguration: cnpgv1.StorageConfiguration{
				StorageClass: &PGStorgeClass,
				Size:         "10Gi",
			},
			PostgresConfiguration: cnpgv1.PostgresConfiguration{
				Parameters: map[string]string{
					"idle_in_transaction_session_timeout": "120000",
					"idle_session_timeout":                "60000",
					"tcp_keepalives_idle":                 "120",
					"tcp_keepalives_interval":             "20",
					"tcp_keepalives_count":                "10",
					"max_connections":                     "300",
				},
				PgHBA: []string{
					"host all all 0.0.0.0/0 trust",
				},
			},
			SuperuserSecret: &cnpgv1.LocalObjectReference{
				Name: PGSuperUserSecretName(cluster),
			},
			EnableSuperuserAccess: &enableSupseruserAccess,
			Bootstrap: &cnpgv1.BootstrapConfiguration{
				InitDB: &cnpgv1.BootstrapInitDB{
					Database: PGInitDBName,
					Secret: &cnpgv1.LocalObjectReference{
						Name: PGInitDBUserSecretName(cluster),
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(cluster, PGDesired, r.Scheme); err != nil {
		return nil, err
	}

	return PGDesired, nil
}

func (r *NineClusterReconciler) reconcilePGCluster(ctx context.Context, cluster *ninev1alpha1.NineCluster, pg ninev1alpha1.ClusterInfo, logger logr.Logger) error {
	err := r.reconcilePGInitDBUserSecret(ctx, cluster, pg)
	if err != nil {
		logger.Error(err, "Failed to reconcilePGInitDBUserSecret")
		return err
	}

	err = r.reconcilePGSuperUserSecret(ctx, cluster, pg)
	if err != nil {
		logger.Error(err, "Failed to reconcilePGSuperUserSecret")
		return err
	}

	desiredPG, err := r.constructPGCluster(ctx, cluster, pg)
	if err != nil {
		logger.Error(err, "Failed to constructPGCluster")
		return err
	}

	metav1.AddToGroupVersion(runtime.NewScheme(), cnpgv1.GroupVersion)
	utilruntime.Must(cnpgv1.AddToScheme(runtime.NewScheme()))

	config, err := GetK8sClientConfig()
	if err != nil {
		return err
	}

	pc, err := poversioned.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = pc.PostgresqlV1().Clusters(cluster.Namespace).Get(context.TODO(), pgResourceName(cluster), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		logger.Info("Start to create a new PGCluster...")
		_, err := pc.PostgresqlV1().Clusters(cluster.Namespace).Create(context.TODO(), desiredPG, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	logger.Info("Reconcile a PGCluster successfully")
	return nil
}
