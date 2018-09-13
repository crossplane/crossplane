/*
Copyright 2018 The Conductor Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Substantial portions of this code is based on https://github.com/sorenmat/k8s-rds, which uses the following license:

MIT License
Copyright (c) 2018 Soren Mathiasen

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
associated documentation files (the "Software"), to deal in the Software without restriction,
including without limitation the rights to use, copy, modify, merge, publish, distribute,
sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or
substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING
BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package rds

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/rdsiface"
	awsv1alpha1 "github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
	awsclients "github.com/upbound/conductor/pkg/clients/aws"
	k8sclients "github.com/upbound/conductor/pkg/clients/kubernetes"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new RDS Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	clientset, err := k8sclients.GetClientset()
	if err != nil {
		return err
	}

	ec2client, err := awsclients.EC2ClientFromClientset(clientset)
	if err != nil {
		return err
	}

	rdsClient := rds.New(ec2client.Config)

	return add(mgr, newReconciler(mgr, clientset, ec2client, rdsClient))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, clientset kubernetes.Interface, ec2client ec2iface.EC2API, rdsClient rdsiface.RDSAPI) reconcile.Reconciler {
	return &ReconcileRDS{
		Client:    mgr.GetClient(),
		Clientset: clientset,
		EC2:       ec2client,
		RDS:       rdsClient,
		scheme:    mgr.GetScheme(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("rds-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to RDS
	log.Printf("watching for changes to RDS instances...")
	err = c.Watch(&source.Kind{Type: &awsv1alpha1.RDS{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileRDS{}

// ReconcileRDS reconciles a RDS object
type ReconcileRDS struct {
	client.Client
	Clientset kubernetes.Interface
	EC2       ec2iface.EC2API
	RDS       rdsiface.RDSAPI
	scheme    *runtime.Scheme
}

// Reconcile reads that state of the cluster for a RDS object and makes changes based on the state read
// and what is in the RDS.Spec
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get
// +kubebuilder:rbac:groups=aws.conductor.io,resources=rdss,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileRDS) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the CRD instance
	instance := &awsv1alpha1.RDS{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Printf("failed to get object at start of reconcile loop: %+v", err)
		return reconcile.Result{}, err
	}

	// search for the RDS instance in AWS
	log.Printf("Trying to find db instance %v\n", instance.Name)
	k := &rds.DescribeDBInstancesInput{DBInstanceIdentifier: aws.String(instance.Name)}
	res := r.RDS.DescribeDBInstancesRequest(k)
	_, err = res.Send()
	if err != nil && strings.Contains(err.Error(), rds.ErrCodeDBInstanceNotFoundFault) {
		// seems like we didn't find a database with this name, let's create one
		log.Printf("DB instance %v not found, will try to create it now...", instance.Name)

		// retrieve all the subnets
		log.Println("trying to get subnets")
		subnets, err := r.getSubnets(instance.Spec.PubliclyAccessible)
		if err != nil {
			err = fmt.Errorf("unable to get subnets from db instance %s: %+v", instance.Name, err)
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}

		// retrieve all the security groups
		log.Println("trying to get security groups")
		securityGroups, err := r.getSecurityGroups()
		if err != nil {
			err = fmt.Errorf("unable to get security groups from db instance %s: %+v", instance.Name, err)
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}

		// ensure that subnets exist for the RDS instance we are going to create
		subnetName, err := r.ensureSubnets(instance, subnets)
		if err != nil {
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}

		password, err := k8sclients.GetSecret(r.Clientset, instance.Namespace, instance.Spec.Password.Name, instance.Spec.Password.Key)
		if err != nil {
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}

		input := convertSpecToInput(instance, subnetName, securityGroups, password)

		// make the call to create the RDS instance in AWS
		log.Printf("creating DB with request: %+v", input)
		res := r.RDS.CreateDBInstanceRequest(input)
		_, err = res.Send()
		if err != nil {
			err = fmt.Errorf("failed call to CreateDBInstance for db instance %s: %+v", instance.Name, err)
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}

		log.Printf("sleep a bit after creating the database...")
		time.Sleep(5 * time.Second)
	} else if err != nil {
		err = fmt.Errorf("wasn't able to describe the db instance with id %s: %+v", instance.Name, err)
		log.Printf("%+v", err)
		return reconcile.Result{}, err
	}

	// wait until the RDS instance is available, this could take a long time if we just created it
	log.Printf("Waiting for db instance %s to become available", instance.Name)
	time.Sleep(250 * time.Millisecond)
	err = r.RDS.WaitUntilDBInstanceAvailable(k)
	if err != nil {
		err = fmt.Errorf("something went wrong in WaitUntilDBInstanceAvailable for db instance %s: %v", instance.Name, err)
		log.Printf("%+v", err)
		return reconcile.Result{}, err
	}

	// retrieve the latest RDS instance from AWS
	rdsInstance, err := r.getCreatedRDSInstance(instance.Name)
	if err != nil {
		log.Printf("%+v", err)
		return reconcile.Result{}, err
	}

	message := fmt.Sprintf("RDS instance %s exists: [%s, %s]", instance.Name, *rdsInstance.DBInstanceIdentifier, *rdsInstance.DBInstanceArn)
	log.Printf("%s", message)

	// update the CRD status now that the RDS instance is created
	instance.Status = awsv1alpha1.RDSStatus{
		Message:    message,
		State:      *rdsInstance.DBInstanceStatus,
		ProviderID: *rdsInstance.DBInstanceArn,
	}
	err = r.Update(context.TODO(), instance)
	if err != nil {
		err = fmt.Errorf("failed to update status of CRD instance %s: %+v", instance.Name, err)
		log.Printf("%+v", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileRDS) ensureSubnets(db *awsv1alpha1.RDS, subnets []string) (string, error) {
	if len(subnets) == 0 {
		return "", fmt.Errorf("Error: unable to continue due to lack of subnets, perhaps we couldn't lookup the subnets")
	}
	subnetDescription := "subnet for " + db.Name + " in namespace " + db.Namespace
	subnetName := db.Name + "-subnet"

	svc := r.RDS

	sf := &rds.DescribeDBSecurityGroupsInput{DBSecurityGroupName: aws.String(subnetName)}
	log.Printf("DescribeDBSecurityGroupsRequest for %+v", sf)
	res := svc.DescribeDBSecurityGroupsRequest(sf)
	_, err := res.Send()
	if err != nil {
		// assume we didn't find it..
		subnet := &rds.CreateDBSubnetGroupInput{
			DBSubnetGroupDescription: aws.String(subnetDescription),
			DBSubnetGroupName:        aws.String(subnetName),
			SubnetIds:                subnets,
			Tags:                     []rds.Tag{{Key: aws.String("DBName"), Value: aws.String(db.Name)}},
		}
		res := svc.CreateDBSubnetGroupRequest(subnet)
		_, err := res.Send()
		if err != nil && !strings.Contains(err.Error(), rds.ErrCodeDBSubnetGroupAlreadyExistsFault) {
			return "", fmt.Errorf("failed call to CreateDBSubnetGroup: %+v", err)
		}
	}

	return subnetName, nil
}

// getSubnets returns a list of subnets that the RDS instance should be attached to
// We do this by finding a node in the cluster, take the VPC id from that node a list
// the security groups in the VPC
func (r *ReconcileRDS) getSubnets(public bool) ([]string, error) {
	name, err := r.getFirstNodeName()
	if err != nil {
		return nil, err
	}

	log.Printf("Taking subnets from node %s", name)
	res, err := r.describeInstances(name)
	if err != nil {
		return nil, err
	}

	var result []string
	if len(res.Reservations) >= 1 {
		vpcID := res.Reservations[0].Instances[0].VpcId
		for _, v := range res.Reservations[0].Instances[0].SecurityGroups {
			log.Printf("Security groupid: %+v", *v.GroupId)
		}
		log.Printf("Found VPC %v will search for subnet in that VPC\n", *vpcID)

		res := r.EC2.DescribeSubnetsRequest(&ec2.DescribeSubnetsInput{Filters: []ec2.Filter{{Name: aws.String("vpc-id"), Values: []string{*vpcID}}}})
		subnets, err := res.Send()
		if err != nil {
			return nil, fmt.Errorf("unable to describe subnet in VPC %v: %+v", *vpcID, err)
		}

		for _, sn := range subnets.Subnets {
			if *sn.MapPublicIpOnLaunch == public {
				result = append(result, *sn.SubnetId)
			} else {
				log.Printf("Skipping subnet %v since it's public state was %v and we were looking for %v\n", *sn.SubnetId, *sn.MapPublicIpOnLaunch, public)
			}
		}

	}
	log.Printf("Found the following subnets: %s", strings.Join(result, ", "))
	return result, nil
}

func (r *ReconcileRDS) getSecurityGroups() ([]string, error) {
	name, err := r.getFirstNodeName()
	if err != nil {
		return nil, err
	}

	log.Printf("Taking security groups from node %s", name)
	res, err := r.describeInstances(name)
	if err != nil {
		return nil, err
	}

	var result []string
	if len(res.Reservations) >= 1 {
		for _, v := range res.Reservations[0].Instances[0].SecurityGroups {
			fmt.Printf("Security groupid: %+v", *v.GroupId)
			result = append(result, *v.GroupId)
		}
	}

	log.Printf("Found the following security groups: %s", strings.Join(result, ", "))
	return result, nil
}

func (r *ReconcileRDS) getFirstNodeName() (string, error) {
	nodes, err := r.Clientset.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to get nodes: %+v", err)
	}

	name := ""
	if len(nodes.Items) > 0 {
		// take the first one, we assume that all nodes are created in the same VPC
		name = nodes.Items[0].Spec.ExternalID
	} else {
		return "", fmt.Errorf("unable to find any nodes in the cluster")
	}

	return name, nil
}

func (r *ReconcileRDS) describeInstances(name string) (*ec2.DescribeInstancesOutput, error) {
	params := &ec2.DescribeInstancesInput{
		Filters: []ec2.Filter{
			{
				Name: aws.String("instance-id"),
				Values: []string{
					name,
				},
			},
		},
	}
	log.Printf("trying to describe instance-id %s", name)
	req := r.EC2.DescribeInstancesRequest(params)
	res, err := req.Send()
	if err != nil {
		return nil, fmt.Errorf("unable to describe AWS instance %+v: %+v", params, err)
	}

	// debug output for details of all returned ec2 instances
	// log.Printf("got instance response: %+v", res)

	return res, nil
}

func (r *ReconcileRDS) getCreatedRDSInstance(name string) (*rds.DBInstance, error) {
	k := &rds.DescribeDBInstancesInput{DBInstanceIdentifier: aws.String(name)}
	res := r.RDS.DescribeDBInstancesRequest(k)
	instance, err := res.Send()
	if err != nil {
		return nil, fmt.Errorf("wasn't able to describe the db instance with id %s. err: %+v", name, err)
	}

	if len(instance.DBInstances) != 1 {
		return nil, fmt.Errorf("expected 1 db instance with id %s. returned instances: %+v", name, instance)
	}

	return &instance.DBInstances[0], nil
}

func convertSpecToInput(db *awsv1alpha1.RDS, subnetName string, securityGroups []string, password string) *rds.CreateDBInstanceInput {
	input := &rds.CreateDBInstanceInput{
		DBName:                aws.String(db.Name),
		AllocatedStorage:      aws.Int64(db.Spec.Size),
		DBInstanceClass:       aws.String(db.Spec.Class),
		DBInstanceIdentifier:  aws.String(db.Name),
		VpcSecurityGroupIds:   securityGroups,
		Engine:                aws.String(db.Spec.Engine),
		MasterUserPassword:    aws.String(string(password)),
		MasterUsername:        aws.String(db.Spec.Username),
		DBSubnetGroupName:     aws.String(subnetName),
		PubliclyAccessible:    aws.Bool(db.Spec.PubliclyAccessible),
		MultiAZ:               aws.Bool(db.Spec.MultiAZ),
		StorageEncrypted:      aws.Bool(db.Spec.StorageEncrypted),
		BackupRetentionPeriod: aws.Int64(db.Spec.BackupRetentionPeriod),
	}
	if db.Spec.StorageType != "" {
		input.StorageType = aws.String(db.Spec.StorageType)
	}
	if db.Spec.Iops > 0 {
		input.Iops = aws.Int64(db.Spec.Iops)
	}
	return input
}
