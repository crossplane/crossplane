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

package s3

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/crossplaneio/crossplane/pkg/apis/aws"
	. "github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	awsv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	client "github.com/crossplaneio/crossplane/pkg/clients/aws/s3"
	. "github.com/crossplaneio/crossplane/pkg/clients/aws/s3/fake"
	"github.com/crossplaneio/crossplane/pkg/util"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	. "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	. "k8s.io/client-go/testing"
	. "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace    = "default"
	providerName = "test-provider"
	bucketName   = "test-bucket"
)

var (
	key = types.NamespacedName{
		Namespace: namespace,
		Name:      bucketName,
	}
	request = reconcile.Request{
		NamespacedName: key,
	}
)

func init() {
	if err := aws.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
}

func testProvider() *awsv1alpha1.Provider {
	return &awsv1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: namespace,
		},
	}
}

func testResource() *S3Bucket {
	perm := storagev1alpha1.ReadOnlyPermission
	testIAMUsername := "test-username"
	return &S3Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bucketName,
			Namespace: namespace,
		},
		Spec: S3BucketSpec{
			LocalPermission: &perm,
		},
		Status: S3BucketStatus{
			IAMUsername: testIAMUsername,
		},
	}
}

// assertResource a helper function to check on cluster and its status
func assertResource(g *GomegaWithT, r *Reconciler, s corev1alpha1.ConditionedStatus) *S3Bucket {
	resource := &S3Bucket{}
	err := r.Get(ctx, key, resource)
	g.Expect(err).To(BeNil())
	g.Expect(resource.Status.ConditionedStatus).Should(corev1alpha1.MatchConditionedStatus(s))
	return resource
}

func TestSyncBucketError(t *testing.T) {
	g := NewGomegaWithT(t)

	assert := func(instance *S3Bucket, client client.Service, expectedResult reconcile.Result, expectedStatus corev1alpha1.ConditionedStatus) {
		r := &Reconciler{
			Client:     NewFakeClient(instance),
			kubeclient: NewSimpleClientset(),
		}

		rs, err := r._sync(instance, client)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(rs).To(Equal(expectedResult))
		assertResource(g, r, expectedStatus)
	}

	// error iam username not set
	testError := "username not set, .Status.IAMUsername"
	cl := &MockS3Client{}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorSyncResource, testError)
	noUserResource := testResource()
	noUserResource.Status.IAMUsername = ""
	assert(noUserResource, cl, resultRequeue, expectedStatus)

	// error get bucket info
	testError = "mock get bucket info err"
	cl.MockGetBucketInfo = func(username string, spec *S3BucketSpec) (*client.Bucket, error) {
		return nil, fmt.Errorf(testError)
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorSyncResource, testError)
	assert(testResource(), cl, resultRequeue, expectedStatus)

	//update versioning error
	cl.MockGetBucketInfo = func(username string, spec *S3BucketSpec) (*client.Bucket, error) {
		return &client.Bucket{Versioning: true, UserPolicyVersion: "v1"}, nil
	}

	testError = "bucket-versioning-update-error"
	cl.MockUpdateVersioning = func(spec *S3BucketSpec) error {
		return fmt.Errorf(testError)
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorSyncResource, testError)
	assert(testResource(), cl, resultRequeue, expectedStatus)

	// update bucket acl error
	cl.MockGetBucketInfo = func(username string, spec *S3BucketSpec) (*client.Bucket, error) {
		return &client.Bucket{Versioning: false, UserPolicyVersion: "v1"}, nil
	}

	testError = "bucket-acl-update-error"
	cl.MockUpdateBucketACL = func(spec *S3BucketSpec) error {
		return fmt.Errorf(testError)
	}

	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorSyncResource, testError)
	assert(testResource(), cl, resultRequeue, expectedStatus)

	cl.MockUpdateBucketACL = func(spec *S3BucketSpec) error {
		return nil
	}

	// Update policy error
	perm := storagev1alpha1.WriteOnlyPermission
	bucketWithPolicyChanges := testResource()
	bucketWithPolicyChanges.Spec.LocalPermission = &perm
	bucketWithPolicyChanges.Status.LastUserPolicyVersion = 1
	bucketWithPolicyChanges.Status.LastLocalPermission = storagev1alpha1.ReadOnlyPermission

	testError = "policy-update-err"
	cl.MockUpdatePolicyDocument = func(username string, spec *S3BucketSpec) (string, error) {
		return "", fmt.Errorf(testError)
	}
	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorSyncResource, testError)
	assert(testResource(), cl, resultRequeue, expectedStatus)
}

func TestSyncCluster(t *testing.T) {
	g := NewGomegaWithT(t)
	tr := testResource()
	tr.Status.LastUserPolicyVersion = 1
	tr.Status.LastLocalPermission = storagev1alpha1.ReadOnlyPermission
	tr.Status.SetFailed("test", "test-msg")

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(),
	}
	//
	updateBucketACLCalled := false
	getBucketInfoCalled := false
	cl := &MockS3Client{
		MockUpdateBucketACL: func(spec *S3BucketSpec) error {
			updateBucketACLCalled = true
			return nil
		},
		MockGetBucketInfo: func(username string, spec *S3BucketSpec) (*client.Bucket, error) {
			getBucketInfoCalled = true
			return &client.Bucket{Versioning: false, UserPolicyVersion: "v1"}, nil
		},
	}

	// Successful Sync should clear failed setatus.
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed("test", "test-msg")
	expectedStatus.UnsetCondition(corev1alpha1.Failed)
	rs, err := r._sync(tr, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(updateBucketACLCalled).To(BeTrue())
	g.Expect(getBucketInfoCalled).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestDelete(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(),
	}

	cl := &MockS3Client{}

	// test delete w/ reclaim policy
	tr.Spec.ReclaimPolicy = corev1alpha1.ReclaimRetain
	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetDeleting()

	rs, err := r._delete(tr, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus)

	// test delete w/ delete policy
	tr.Spec.ReclaimPolicy = corev1alpha1.ReclaimDelete
	called := false
	cl.MockDelete = func(bucket *S3Bucket) error {
		called = true
		return nil
	}

	rs, err = r._delete(tr, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)

	// test delete w/ delete policy error
	testError := "test-delete-error"
	called = false
	cl.MockDelete = func(bucket *S3Bucket) error {
		called = true
		return fmt.Errorf(testError)
	}
	expectedStatus.SetFailed(errorDeleteResource, testError)

	rs, err = r._delete(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestCreate(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()

	tk := NewSimpleClientset()

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: tk,
	}

	createOrUpdateBucketCalled := false
	createUserCalled := false
	cl := &MockS3Client{
		MockCreateUser: func(username string, spec *S3BucketSpec) (*iam.AccessKey, string, error) {
			createUserCalled = true
			fakeKey := &iam.AccessKey{
				AccessKeyId:     util.String("fake-string"),
				SecretAccessKey: util.String(""),
			}
			return fakeKey, "v2", nil
		},
		MockCreateOrUpdateBucket: func(spec *S3BucketSpec) error {
			createOrUpdateBucketCalled = true
			return nil
		},
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetReady()

	resource := testResource()
	rs, err := r._create(resource, cl)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(createOrUpdateBucketCalled).To(BeTrue())
	g.Expect(createUserCalled).To(BeTrue())
	assertResource(g, r, expectedStatus)
	g.Expect(resource.Status.LastUserPolicyVersion).To(Equal(2))

	// assertSecret
	g.Expect(tk.Actions()).To(HaveLen(2))
	g.Expect(tk.Actions()[0].GetVerb()).To(Equal("get"))
	g.Expect(tk.Actions()[1].GetVerb()).To(Equal("create"))
	s, err := tk.CoreV1().Secrets(tr.Namespace).Get(tr.Name, metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(s).NotTo(BeNil())
}

func TestCreateFail(t *testing.T) {
	g := NewGomegaWithT(t)
	tr := testResource()
	tk := NewSimpleClientset()
	cl := &MockS3Client{
		MockCreateUser: func(username string, spec *S3BucketSpec) (*iam.AccessKey, string, error) {
			fakeKey := &iam.AccessKey{
				AccessKeyId:     util.String("fake-string"),
				SecretAccessKey: util.String(""),
			}
			return fakeKey, "v2", nil
		},
		MockCreateOrUpdateBucket: func(spec *S3BucketSpec) error {
			return nil
		},
	}

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: tk,
	}

	// test apply secret error
	testError := "test-get-secret-error"
	tk.PrependReactor("get", "secrets", func(action Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf(testError)
	})

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorCreateResource, testError)

	rs, err := r._create(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	assertResource(g, r, expectedStatus)

	// test create resource error
	tr = testResource()
	r.kubeclient = NewSimpleClientset()
	testError = "test-create-user--error"
	called := false
	cl.MockCreateUser = func(username string, spec *S3BucketSpec) (*iam.AccessKey, string, error) {
		called = true
		return nil, "", fmt.Errorf(testError)
	}

	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorCreateResource, testError)

	rs, err = r._create(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)

	// test create bucket error
	cl.MockCreateUser = func(username string, spec *S3BucketSpec) (*iam.AccessKey, string, error) {
		fakeKey := &iam.AccessKey{
			AccessKeyId:     util.String("fake-string"),
			SecretAccessKey: util.String(""),
		}
		return fakeKey, "v2", nil
	}

	tr = testResource()
	r.kubeclient = NewSimpleClientset()
	testError = "test-create-bucket--error"
	called = false
	cl.MockCreateOrUpdateBucket = func(spec *S3BucketSpec) error {
		called = true
		return fmt.Errorf(testError)
	}

	expectedStatus = corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorCreateResource, testError)

	rs, err = r._create(tr, cl)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)
}

func TestConnect(t *testing.T) {
	g := NewGomegaWithT(t)

	tp := testProvider()
	tr := testResource()

	r := &Reconciler{
		Client:     NewFakeClient(tp),
		kubeclient: NewSimpleClientset(),
	}

	// provider status is not ready
	c, err := r._connect(tr)
	g.Expect(c).To(BeNil())
	g.Expect(err).To(And(Not(BeNil()), MatchError("provider is not ready")))

	// error getting aws config - secret is not found
	tp.Status.SetReady()
	r.Client = NewFakeClient(tp)
	c, err = r._connect(tr)
	g.Expect(c).To(BeNil())
	g.Expect(err).To(Not(BeNil()))
}

func TestReconcile(t *testing.T) {
	g := NewGomegaWithT(t)

	tr := testResource()
	tr.Status.IAMUsername = ""

	r := &Reconciler{
		Client:     NewFakeClient(tr),
		kubeclient: NewSimpleClientset(),
	}

	// test connect error
	called := false
	testError := "test-connect-error"
	r.connect = func(*S3Bucket) (client.Service, error) {
		called = true
		return nil, fmt.Errorf(testError)
	}

	expectedStatus := corev1alpha1.ConditionedStatus{}
	expectedStatus.SetFailed(errorResourceClient, testError)

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(called).To(BeTrue())
	assertResource(g, r, expectedStatus)

	// test delete
	r.connect = func(instance *S3Bucket) (client client.Service, e error) {
		t := metav1.Now()
		instance.DeletionTimestamp = &t
		return nil, nil
	}
	called = false
	r.delete = func(instance *S3Bucket, client client.Service) (i reconcile.Result, e error) {
		called = true
		return result, nil
	}
	rs, err = r.Reconcile(request)
	g.Expect(called).To(BeTrue())

	// test create
	r.connect = func(instance *S3Bucket) (client client.Service, e error) {
		return nil, nil
	}
	called = false
	r.delete = r._delete
	r.create = func(instance *S3Bucket, client client.Service) (i reconcile.Result, e error) {
		called = true
		return result, nil
	}
	rs, err = r.Reconcile(request)
	g.Expect(called).To(BeTrue())

	// test sync
	r.connect = func(instance *S3Bucket) (client client.Service, e error) {
		instance.Status.IAMUsername = "foo-user"
		return nil, nil
	}
	called = false
	r.create = r._create
	r.sync = func(instance *S3Bucket, client client.Service) (i reconcile.Result, e error) {
		called = true
		return result, nil
	}
	rs, err = r.Reconcile(request)
	g.Expect(called).To(BeTrue())
}
