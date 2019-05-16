package s3

import (
	"errors"
	"testing"

	awsstorage "github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	storage "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	fakeiam "github.com/crossplaneio/crossplane/pkg/clients/aws/iam/fake"
	fakeops "github.com/crossplaneio/crossplane/pkg/clients/aws/s3/operations/fake"

	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/stretchr/testify/mock"
)

func TestNewClient(t *testing.T) {

}

func TestClient_CreateOrUpdateBucket(t *testing.T) {
	ownedErr := awserr.New(s3.ErrCodeBucketAlreadyOwnedByYou, "", nil)

	// Define test cases
	tests := map[string]struct {
		bucket          *awsstorage.S3Bucket
		createBucketRet []interface{}
		putBucketACLRet []interface{}
		ret             []types.GomegaMatcher
	}{
		"HappyPath": {
			bucket:          &awsstorage.S3Bucket{},
			createBucketRet: []interface{}{nil, nil},
			putBucketACLRet: []interface{}{nil, nil},
			ret:             []types.GomegaMatcher{gomega.BeNil()},
		},
		"CreateBucketError": {
			bucket:          &awsstorage.S3Bucket{},
			createBucketRet: []interface{}{nil, ownedErr},
			putBucketACLRet: []interface{}{nil, nil},
			ret:             []types.GomegaMatcher{gomega.BeNil()},
		},
	}

	for testName, vals := range tests {
		t.Run(testName, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			// Set up mocks
			createBucketReq := new(fakeops.CreateBucketRequest)
			createBucketReq.On("Send").Return(vals.createBucketRet...)

			putBucketACLReq := new(fakeops.PutBucketACLRequest)
			putBucketACLReq.On("Send").Return(vals.putBucketACLRet...)

			ops := new(fakeops.Operations)
			ops.On("CreateBucketRequest", mock.Anything).Return(createBucketReq)
			ops.On("PutBucketACLRequest", mock.Anything).Return(putBucketACLReq)

			// Create thing we are testing
			c := Client{s3: ops}

			// Call the method under test
			err := c.CreateOrUpdateBucket(vals.bucket)

			// Make assertions
			g.Expect(err).To(vals.ret[0])
		})
	}
}

func TestClient_GetBucketInfo(t *testing.T) {
	// Set up args
	name := "han"
	s3Bucket := &awsstorage.S3Bucket{}
	versioningOut := new(s3.GetBucketVersioningOutput)
	boom := errors.New("boom")

	// Define test cases
	tests := map[string]struct {
		sendErr             error
		getPolicyVersionErr error
		bucketInfoRet1      types.GomegaMatcher
		bucketInfoRet2      types.GomegaMatcher
	}{
		"HappyPath": {
			sendErr:             nil,
			getPolicyVersionErr: nil,
			bucketInfoRet1:      gomega.Not(gomega.BeNil()),
			bucketInfoRet2:      gomega.BeNil(),
		},
		"SendError": {
			sendErr:             boom,
			getPolicyVersionErr: nil,
			bucketInfoRet1:      gomega.BeNil(),
			bucketInfoRet2:      gomega.Equal(boom),
		},
		"IAMError": {
			sendErr:             nil,
			getPolicyVersionErr: boom,
			bucketInfoRet1:      gomega.BeNil(),
			bucketInfoRet2:      gomega.Equal(boom),
		},
	}

	for testName, vals := range tests {
		t.Run(testName, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			// Set up mocks
			versioningReq := new(fakeops.GetBucketVersioningRequest)
			versioningReq.On("Send").Return(versioningOut, vals.sendErr)

			ops := new(fakeops.Operations)
			ops.On("GetBucketVersioningRequest", mock.Anything).Return(versioningReq)

			iamc := new(fakeiam.Client)
			iamc.On("GetPolicyVersion", name).Return("han-is-cool", vals.getPolicyVersionErr)

			// Create thing we are testing
			c := Client{s3: ops, iamClient: iamc}

			// Call the method under test
			b, err := c.GetBucketInfo(name, s3Bucket)

			// Make assertions
			g.Expect(err).To(vals.bucketInfoRet2)
			g.Expect(b).To(vals.bucketInfoRet1)
		})
	}
}

func TestClient_CreateUser(t *testing.T) {
	// Set up shared args
	boom := errors.New("boom")
	name := "han"
	key := &iam.AccessKey{}
	version := "v1.0.0"
	fakePerm := storage.LocalPermissionType("fake")

	// Define test cases
	tests := map[string]struct {
		s3Bucket        *awsstorage.S3Bucket
		createUserRet   []interface{}
		createPolicyRet []interface{}
		ret             []types.GomegaMatcher
	}{
		"HappyPath": {
			s3Bucket:        &awsstorage.S3Bucket{},
			createUserRet:   []interface{}{key, nil},
			createPolicyRet: []interface{}{version, nil},
			ret:             []types.GomegaMatcher{gomega.Equal(key), gomega.Equal(version), gomega.BeNil()},
		},
		"BadBucket": {
			s3Bucket:        &awsstorage.S3Bucket{Spec: awsstorage.S3BucketSpec{LocalPermission: &fakePerm}},
			createUserRet:   []interface{}{key, nil},
			createPolicyRet: []interface{}{version, nil},
			ret:             []types.GomegaMatcher{gomega.BeNil(), gomega.Equal(""), gomega.Equal(errors.New("could not update policy, unknown permission, fake"))},
		},
		"IAMCreateUserError": {
			s3Bucket:        &awsstorage.S3Bucket{},
			createUserRet:   []interface{}{nil, boom},
			createPolicyRet: []interface{}{version, nil},
			ret:             []types.GomegaMatcher{gomega.BeNil(), gomega.Equal(""), gomega.Equal(errors.New("could not create user boom"))},
		},
		"IAMCreatePolicyError": {
			s3Bucket:        &awsstorage.S3Bucket{},
			createUserRet:   []interface{}{key, nil},
			createPolicyRet: []interface{}{"", boom},
			ret:             []types.GomegaMatcher{gomega.BeNil(), gomega.Equal(""), gomega.Equal(errors.New("could not create policy boom"))},
		},
	}

	for testName, vals := range tests {
		t.Run(testName, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			// Set up mocks
			iamc := new(fakeiam.Client)
			iamc.On("CreateUser", name).Return(vals.createUserRet...)
			iamc.On("CreatePolicyAndAttach", name, name, mock.Anything).Return(vals.createPolicyRet...)

			// Create thing we are testing
			c := Client{iamClient: iamc}

			// Call the method under test
			key, version, err := c.CreateUser(name, vals.s3Bucket)

			// Make assertions
			g.Expect(key).To(vals.ret[0])
			g.Expect(version).To(vals.ret[1])
			g.Expect(err).To(vals.ret[2])

		})
	}
}

func TestClient_UpdateBucketACL(t *testing.T) {
	acl := s3.BucketCannedACLPrivate

	// Define test cases
	tests := map[string]struct {
		bucket  *awsstorage.S3Bucket
		sendRet []interface{}
		ret     []types.GomegaMatcher
	}{
		"HappyPath": {
			bucket:  &awsstorage.S3Bucket{},
			sendRet: []interface{}{&s3.PutBucketAclOutput{}, nil},
			ret:     []types.GomegaMatcher{gomega.BeNil()},
		},
		"WithACL": {
			bucket:  &awsstorage.S3Bucket{Spec: awsstorage.S3BucketSpec{CannedACL: &acl}},
			sendRet: []interface{}{&s3.PutBucketAclOutput{}, nil},
			ret:     []types.GomegaMatcher{gomega.BeNil()},
		},
	}

	for testName, vals := range tests {
		t.Run(testName, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			// Set up mocks
			putBucketACL := new(fakeops.PutBucketACLRequest)
			putBucketACL.On("Send").Return(vals.sendRet...)

			ops := new(fakeops.Operations)
			ops.On("PutBucketACLRequest", mock.Anything).Return(putBucketACL)

			// Create thing we are testing
			c := Client{s3: ops}

			// Call the method under test
			err := c.UpdateBucketACL(vals.bucket)

			// Make assertions
			g.Expect(err).To(vals.ret[0])
		})
	}
}

func TestClient_UpdateVersioning(t *testing.T) {
	boom := errors.New("boom")
	// Define test cases
	tests := map[string]struct {
		bucket  *awsstorage.S3Bucket
		sendRet []interface{}
		ret     []types.GomegaMatcher
	}{
		"HappyPath": {
			bucket:  &awsstorage.S3Bucket{Spec: awsstorage.S3BucketSpec{Versioning: true}},
			sendRet: []interface{}{&s3.PutBucketVersioningOutput{}, nil},
			ret:     []types.GomegaMatcher{gomega.BeNil()},
		},
		"SendError": {
			bucket:  &awsstorage.S3Bucket{},
			sendRet: []interface{}{&s3.PutBucketVersioningOutput{}, boom},
			ret:     []types.GomegaMatcher{gomega.Equal(boom)},
		},
	}

	for testName, vals := range tests {
		t.Run(testName, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			// Set up mocks
			putBucketVer := new(fakeops.PutBucketVersioningRequest)
			putBucketVer.On("Send").Return(vals.sendRet...)

			ops := new(fakeops.Operations)
			ops.On("PutBucketVersioningRequest", mock.Anything).Return(putBucketVer)

			// Create thing we are testing
			c := Client{s3: ops}

			// Call the method under test
			err := c.UpdateVersioning(vals.bucket)

			// Make assertions
			g.Expect(err).To(vals.ret[0])
		})
	}
}

func TestClient_UpdatePolicyDocument(t *testing.T) {
	boom := errors.New("boom")
	user := "han"
	ver := "version"
	fakePerm := storage.LocalPermissionType("fake")

	// Define test cases
	tests := map[string]struct {
		bucket    *awsstorage.S3Bucket
		updateRet []interface{}
		ret       []types.GomegaMatcher
	}{
		"HappyPath": {
			bucket:    &awsstorage.S3Bucket{},
			updateRet: []interface{}{ver, nil},
			ret:       []types.GomegaMatcher{gomega.Equal(ver), gomega.BeNil()},
		},
		"BadBucket": {
			bucket:    &awsstorage.S3Bucket{Spec: awsstorage.S3BucketSpec{LocalPermission: &fakePerm}},
			updateRet: []interface{}{ver, nil},
			ret:       []types.GomegaMatcher{gomega.Equal(""), gomega.Equal(errors.New("could not generate policy, unknown permission, fake"))},
		},
		"IAMUpdateError": {
			bucket:    &awsstorage.S3Bucket{},
			updateRet: []interface{}{"", boom},
			ret:       []types.GomegaMatcher{gomega.Equal(""), gomega.Equal(errors.New("could not update policy, boom"))},
		},
	}

	for testName, vals := range tests {
		t.Run(testName, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			// Set up mocks
			iamc := new(fakeiam.Client)
			iamc.On("UpdatePolicy", user, mock.AnythingOfType("string")).Return(vals.updateRet...)

			// Create thing we are testing
			c := Client{iamClient: iamc}

			// Call the method under test
			ver, err := c.UpdatePolicyDocument(user, vals.bucket)

			// Make assertions
			g.Expect(ver).To(vals.ret[0])
			g.Expect(err).To(vals.ret[1])
		})
	}
}

func TestClient_DeleteBucket(t *testing.T) {
	boom := errors.New("boom")
	user := "han"

	// Define test cases
	tests := map[string]struct {
		bucket          *awsstorage.S3Bucket
		deleteBucketRet []interface{}
		deletePolicyRet []interface{}
		deleteUserRet   []interface{}
		ret             []types.GomegaMatcher
	}{
		"HappyPath": {
			bucket:          &awsstorage.S3Bucket{Status: awsstorage.S3BucketStatus{IAMUsername: user}},
			deleteBucketRet: []interface{}{nil, nil},
			deletePolicyRet: []interface{}{nil},
			deleteUserRet:   []interface{}{nil},
			ret:             []types.GomegaMatcher{gomega.BeNil()},
		},
		"NoUserName": {
			bucket:          &awsstorage.S3Bucket{Status: awsstorage.S3BucketStatus{}},
			deleteBucketRet: []interface{}{nil, nil},
			deletePolicyRet: []interface{}{nil},
			deleteUserRet:   []interface{}{nil},
			ret:             []types.GomegaMatcher{gomega.BeNil()},
		},
		"SendError": {
			bucket:          &awsstorage.S3Bucket{Status: awsstorage.S3BucketStatus{IAMUsername: user}},
			deleteBucketRet: []interface{}{nil, boom},
			deletePolicyRet: []interface{}{nil},
			deleteUserRet:   []interface{}{nil},
			ret:             []types.GomegaMatcher{gomega.Equal(boom)},
		},
		"DeletePolicyError": {
			bucket:          &awsstorage.S3Bucket{Status: awsstorage.S3BucketStatus{IAMUsername: user}},
			deleteBucketRet: []interface{}{nil, nil},
			deletePolicyRet: []interface{}{boom},
			deleteUserRet:   []interface{}{nil},
			ret:             []types.GomegaMatcher{gomega.Equal(boom)},
		},
	}

	for testName, vals := range tests {
		t.Run(testName, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			// Set up mocks
			delBucketReq := new(fakeops.DeleteBucketRequest)
			delBucketReq.On("Send").Return(vals.deleteBucketRet...)

			ops := new(fakeops.Operations)
			ops.On("DeleteBucketRequest", mock.Anything).Return(delBucketReq)

			iamc := new(fakeiam.Client)
			iamc.On("DeletePolicyAndDetach", user, user).Return(vals.deletePolicyRet...)
			iamc.On("DeleteUser", user).Return(vals.deleteUserRet...)

			// Create thing we are testing
			c := Client{s3: ops, iamClient: iamc}

			// Call the method under test
			err := c.DeleteBucket(vals.bucket)

			// Make assertions
			g.Expect(err).To(vals.ret[0])
		})
	}
}

func Test_isErrorAlreadyExists(t *testing.T) {
	tests := map[string]struct {
		input  error
		output bool
	}{
		"GenericError": {
			input:  errors.New("boom"),
			output: false,
		},
		"RightTypeWrongCode": {
			input:  awserr.New("fake", "", nil),
			output: false,
		},
		"RightTypeRightCode": {
			input:  awserr.New(s3.ErrCodeBucketAlreadyOwnedByYou, "", nil),
			output: true,
		},
	}

	for testName, vals := range tests {
		t.Run(testName, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			res := isErrorAlreadyExists(vals.input)
			g.Expect(res).To(gomega.Equal(vals.output))
		})
	}
}

func Test_isErrorNotFound(t *testing.T) {
	tests := map[string]struct {
		input  error
		output bool
	}{
		"GenericError": {
			input:  errors.New("boom"),
			output: false,
		},
		"RightTypeWrongCode": {
			input:  awserr.New("fake", "", nil),
			output: false,
		},
		"RightTypeRightCode": {
			input:  awserr.New(s3.ErrCodeNoSuchBucket, "", nil),
			output: true,
		},
	}

	for testName, vals := range tests {
		t.Run(testName, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			res := isErrorNotFound(vals.input)
			g.Expect(res).To(gomega.Equal(vals.output))
		})
	}
}

func TestCreateBucketInput(t *testing.T) {
	acl := s3.BucketCannedACLPrivate

	// Define test cases
	tests := map[string]struct {
		bucket *awsstorage.S3Bucket
		ret    *s3.CreateBucketInput
	}{
		"NoRegionNoACL": {
			bucket: &awsstorage.S3Bucket{Spec: awsstorage.S3BucketSpec{}},
			ret:    &s3.CreateBucketInput{Bucket: new(string), CreateBucketConfiguration: &s3.CreateBucketConfiguration{}},
		},
		"NoRegionHasACL": {
			bucket: &awsstorage.S3Bucket{Spec: awsstorage.S3BucketSpec{CannedACL: &acl}},
			ret:    &s3.CreateBucketInput{Bucket: new(string), CreateBucketConfiguration: &s3.CreateBucketConfiguration{}, ACL: acl},
		},
		"USEast1NoACL": {
			bucket: &awsstorage.S3Bucket{Spec: awsstorage.S3BucketSpec{Region: regionWithNoConstraint}},
			ret:    &s3.CreateBucketInput{Bucket: new(string)},
		},
		"USEast1HasACL": {
			bucket: &awsstorage.S3Bucket{Spec: awsstorage.S3BucketSpec{Region: regionWithNoConstraint, CannedACL: &acl}},
			ret:    &s3.CreateBucketInput{Bucket: new(string), ACL: acl},
		},
		"USWest2NoACL": {
			bucket: &awsstorage.S3Bucket{Spec: awsstorage.S3BucketSpec{Region: "us-west-2"}},
			ret:    &s3.CreateBucketInput{Bucket: new(string), CreateBucketConfiguration: &s3.CreateBucketConfiguration{LocationConstraint: "us-west-2"}},
		},
	}

	for testName, vals := range tests {
		t.Run(testName, func(t *testing.T) {
			g := gomega.NewGomegaWithT(t)

			// Call the method under test
			res := CreateBucketInput(vals.bucket)

			// Make assertions
			g.Expect(res.Bucket).To(gomega.Equal(vals.ret.Bucket))
			g.Expect(res.CreateBucketConfiguration).To(gomega.Equal(vals.ret.CreateBucketConfiguration))
			g.Expect(res.ACL).To(gomega.Equal(vals.ret.ACL))
		})
	}
}

func TestGenerateBucketUsername(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	b := &awsstorage.S3Bucket{}
	res := GenerateBucketUsername(b)

	g.Expect(res).To(gomega.HavePrefix("crossplane-bucket-"))
}

func Test_newPolicyDocument(t *testing.T) {

}
