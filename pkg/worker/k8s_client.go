package worker

import (
	"github.com/golang/glog"

	"fmt"
	"strconv"

	crds "github.com/MYOB-Technology/ops-kube-db-operator/pkg/apis/postgresdb/v1alpha1"
	postgresdbv1alpha1 "github.com/MYOB-Technology/ops-kube-db-operator/pkg/client/clientset/versioned/typed/postgresdb/v1alpha1"
	"github.com/MYOB-Technology/ops-kube-db-operator/pkg/postgres"
	"github.com/MYOB-Technology/ops-kube-db-operator/pkg/rds"
	"github.com/MYOB-Technology/ops-kube-db-operator/pkg/secret"
	"k8s.io/client-go/kubernetes"
)

const OperatorAdminNamespace = "kube-system"

// K8sClient is a client that wraps all interactions with the k8s api server.
type K8sClient struct {
	// injected deps for testing
	clientset    kubernetes.Interface
	crdClientset postgresdbv1alpha1.PostgresdbV1alpha1Interface
}

// NewK8sClient returns a new K8sClient for interacting with the k8s api server.
func NewK8sClient(clientSet kubernetes.Interface, crdClientset postgresdbv1alpha1.PostgresdbV1alpha1Interface) *K8sClient {
	return &K8sClient{clientset: clientSet, crdClientset: crdClientset}
}

// UpdateCRDStatus updates the Ready message on postgresDB crd with provided status
func (c *K8sClient) UpdateCRDStatus(crd *crds.PostgresDB, namespace, status string) (*crds.PostgresDB, error) {
	crd.Status.Ready = status

	return c.crdClientset.PostgresDBs(namespace).Update(crd)
}

// UpdateCRDStatus updates the postgresDB crd status indicating it is available
func (c *K8sClient) UpdateCRDAsAvailable(crd *crds.PostgresDB, namespace, status, arn string) (*crds.PostgresDB, error) {
	crd.Status.ARN = arn
	return c.UpdateCRDStatus(crd, namespace, status)
}

// SaveAdminSecret save k8s secret with master db user credentials
func (c *K8sClient) SaveMasterSecret(crdName string, masterUser *postgres.User, instance *rds.CreateInstanceOutput, instanceName string) (*secret.DBSecret, error) {
	secretName := fmt.Sprintf("%s-%s", crdName, "master")
	dd := &postgres.DatabaseDescriptor{Database: &postgres.Database{"postgres"}}

	if nil != instance {
		dd.Host = instance.Address
		dd.Port = int(instance.Port)
	}

	return c.saveSecret(secretName, OperatorAdminNamespace, masterUser, dd, instanceName)
}

// SaveAdminSecret save k8s secret with admin db user credentials
func (c *K8sClient) SaveAdminSecret(crd *crds.PostgresDB, dd *postgres.DatabaseDescriptor, instanceName string) (*secret.DBSecret, error) {
	secretName := fmt.Sprintf("%s-%s", crd.ObjectMeta.Name, "admin")

	return c.saveSecret(secretName, crd.ObjectMeta.Namespace, dd.Admin, dd, instanceName)
}

// SaveAdminSecret save k8s secret with metrics db exporter user credentials
func (c *K8sClient) SaveMetricsExporterSecret(crd *crds.PostgresDB, dd *postgres.DatabaseDescriptor, instanceName string) (*secret.DBSecret, error) {
	secretName := fmt.Sprintf("%s-%s", crd.ObjectMeta.Name, "metrics-exporter")
	shadowNamespace := fmt.Sprintf("%s-shadow", crd.ObjectMeta.Namespace)

	return c.saveSecret(secretName, shadowNamespace, dd.MetricsExporter, dd, instanceName)
}

func (c *K8sClient) saveSecret(secretName, namespace string, user *postgres.User, dd *postgres.DatabaseDescriptor, instanceName string) (*secret.DBSecret, error) {
	glog.Infof("Creating %s secret in namespace %s for database instance %s...", user.Name, namespace, instanceName)
	secret, _ := secret.NewOrGet(c.clientset.CoreV1(), namespace, secretName)
	secret.InstanceName = instanceName
	secret.Host = dd.Host
	secret.Port = strconv.Itoa(dd.Port)
	secret.DatabaseName = dd.Database.Name
	secret.Username = user.Name
	secret.Password = user.Password

	glog.Infof("Saving secret %s...", secret)
	err := secret.Save()

	return secret, err
}
