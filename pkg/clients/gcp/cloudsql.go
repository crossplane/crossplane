/*
Copyright 2018 The Crossplane Authors.

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

package gcp

import (
	"fmt"
	"log"
	"time"

	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	dbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
)

// CloudSQLAPI provides an interface for operations on CloudSQL instances
type CloudSQLAPI interface {
	GetInstance(project string, instance string) (*sqladmin.DatabaseInstance, error)
	CreateInstance(project string, databaseinstance *sqladmin.DatabaseInstance) (*sqladmin.Operation, error)
	DeleteInstance(project string, instance string) (*sqladmin.Operation, error)
	ListUsers(project string, instance string) (*sqladmin.UsersListResponse, error)
	UpdateUser(project string, instance string, name string, user *sqladmin.User) (*sqladmin.Operation, error)
	GetOperation(project string, operationID string) (*sqladmin.Operation, error)
}

// CloudSQLClient implements the CloudSQLAPI interface for real CloudSQL instances
type CloudSQLClient struct {
	*sqladmin.Service
}

// NewCloudSQLClient creates a new instance of a CloudSQLClient
func NewCloudSQLClient(clientset kubernetes.Interface, namespace string, secretKey v1.SecretKeySelector) (*CloudSQLClient, error) {
	hc, err := GetGoogleClient(clientset, namespace, secretKey, sqladmin.SqlserviceAdminScope)
	if err != nil {
		return nil, err
	}

	service, err := sqladmin.New(hc)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqladmin client: %+v", err)
	}

	return &CloudSQLClient{service}, nil
}

// GetInstance retrieves details for the requested CloudSQL instance
func (c *CloudSQLClient) GetInstance(project string, instance string) (*sqladmin.DatabaseInstance, error) {
	return c.Instances.Get(project, instance).Do()
}

// CreateInstance creates the given CloudSQL instance
func (c *CloudSQLClient) CreateInstance(project string, databaseinstance *sqladmin.DatabaseInstance) (*sqladmin.Operation, error) {
	return c.Instances.Insert(project, databaseinstance).Do()
}

// DeleteInstance deletes the given CloudSQL instance
func (c *CloudSQLClient) DeleteInstance(project string, instance string) (*sqladmin.Operation, error) {
	return c.Instances.Delete(project, instance).Do()
}

// ListUsers lists all the users for the given CloudSQL instance
func (c *CloudSQLClient) ListUsers(project string, instance string) (*sqladmin.UsersListResponse, error) {
	return c.Users.List(project, instance).Do()
}

// UpdateUser updates the given user for the given CloudSQL instance
func (c *CloudSQLClient) UpdateUser(project string, instance string, name string, user *sqladmin.User) (*sqladmin.Operation, error) {
	return c.Users.Update(project, instance, name, user).Do()
}

// GetOperation retrieves the latest status for the given operation
func (c *CloudSQLClient) GetOperation(project string, operationID string) (*sqladmin.Operation, error) {
	return c.Operations.Get(project, operationID).Do()
}

// CloudSQLAPIFactory defines an interface for creating instances of the CloudSQLAPI interface.
type CloudSQLAPIFactory interface {
	CreateAPIInstance(kubernetes.Interface, string, v1.SecretKeySelector) (CloudSQLAPI, error)
}

// CloudSQLClientFactory will create a real CloudSQL client that talks to GCP
type CloudSQLClientFactory struct {
}

// CreateAPIInstance instantiates a real CloudSQL client that talks to GCP
func (c *CloudSQLClientFactory) CreateAPIInstance(clientset kubernetes.Interface, namespace string,
	secretKey v1.SecretKeySelector) (CloudSQLAPI, error) {

	cloudSQLClient, err := NewCloudSQLClient(clientset, namespace, secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get CloudSQL client: %+v", err)
	}

	return cloudSQLClient, nil
}

// WaitUntilOperationCompletes waits until the supplied operation is complete,
// returning an error if it does not complete within the supplied duration.
func WaitUntilOperationCompletes(operationID string, provider *gcpv1alpha1.Provider,
	cloudSQLClient CloudSQLAPI, waitTime time.Duration) (*sqladmin.Operation, error) {

	var err error
	var op *sqladmin.Operation

	maxRetries := 50
	for i := 0; i <= maxRetries; i++ {
		op, err = cloudSQLClient.GetOperation(provider.Spec.ProjectID, operationID)
		if err != nil {
			log.Printf("failed to get cloud sql operation %s, waiting %v: %+v", operationID, waitTime, err)
		} else if IsOperationComplete(op) {
			// the operation has completed, simply return it
			return op, nil
		}

		<-time.After(waitTime)
	}

	return nil, fmt.Errorf("cloud sql operation %s did not complete in the allowed time period: %+v", operationID, op)
}

// IsOperationComplete returns true if the supplied operation is complete.
func IsOperationComplete(op *sqladmin.Operation) bool {
	return op.EndTime != "" && op.Status == "DONE"
}

// IsOperationSuccessful returns true if the supplied operation was successful.
func IsOperationSuccessful(op *sqladmin.Operation) bool {
	return op.Error == nil || len(op.Error.Errors) == 0
}

// CloudSQLConditionType converts the given CloudSQL state string into a corresponding condition type
func CloudSQLConditionType(state string) corev1alpha1.ConditionType {
	switch state {
	case dbv1alpha1.StateRunnable:
		return corev1alpha1.Ready
	case dbv1alpha1.StatePendingCreate:
		return corev1alpha1.Creating
	default:
		return corev1alpha1.Failed
	}
}

// CloudSQLStatusMessage returns a status message based on the state of the given instance
func CloudSQLStatusMessage(instanceName string, cloudSQLInstance *sqladmin.DatabaseInstance) string {
	if cloudSQLInstance == nil {
		return fmt.Sprintf("Cloud SQL instance %s has not yet been created", instanceName)
	}

	switch cloudSQLInstance.State {
	case dbv1alpha1.StateRunnable:
		return fmt.Sprintf("Cloud SQL instance %s is running", instanceName)
	case dbv1alpha1.StatePendingCreate:
		return fmt.Sprintf("Cloud SQL instance %s is being created", instanceName)
	case dbv1alpha1.StateFailed:
		return fmt.Sprintf("Cloud SQL instance %s failed to be created", instanceName)
	default:
		return fmt.Sprintf("Cloud SQL instance %s is in an unknown state %s", instanceName, cloudSQLInstance.State)
	}
}
