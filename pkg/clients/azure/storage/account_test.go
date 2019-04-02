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

package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-test/deep"
	"github.com/pkg/errors"
)

func TestNewStorageAccountClient(t *testing.T) {
	tests := []struct {
		name    string
		args    []byte
		wantRes *storage.AccountsClient
		wantErr error
	}{
		{
			name:    "empty-data",
			args:    []byte{},
			wantRes: nil,
			wantErr: errors.WithStack(errors.New("cannot unmarshal Azure client secret data: unexpected end of JSON input")),
		},
		{
			name: "success",
			args: []byte(`{"clientId": "0f32e96b-b9a4-49ce-a857-243a33b20e5c",
	"clientSecret": "49d8cab5-d47a-4d1a-9133-5c5db29c345d",
	"subscriptionId": "bf1b0e59-93da-42e0-82c6-5a1d94227911",
	"tenantId": "302de427-dba9-4452-8583-a4268e46de6b",
	"activeDirectoryEndpointUrl": "https://login.microsoftonline.com",
	"resourceManagerEndpointUrl": "https://management.azure.com/",
	"activeDirectoryGraphResourceId": "https://graph.windows.net/",
	"sqlManagementEndpointUrl": "https://management.core.windows.net:8443/",
	"galleryEndpointUrl": "https://gallery.azure.com/",
	"managementEndpointUrl": "https://management.core.windows.net/"}`),
			wantRes: &storage.AccountsClient{},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewStorageAccountClient(tt.args)
			if diff := deep.Equal(err, tt.wantErr); diff != nil {
				t.Errorf("NewStorageAccountClient() error = %v, wantErr %v\n%s", err, tt.wantErr, diff)
			}
			if err != nil && got != nil {
				t.Errorf("NewStorageAccountClient() %v, want nil", got)
			}
			if err == nil && got == nil {
				t.Errorf("NewStorageAccountClient() %v, want not nil", got)
			}
		})
	}
}

func TestNewAccountHandle(t *testing.T) {
	type args struct {
		client      *storage.AccountsClient
		groupName   string
		accountName string
	}
	tests := []struct {
		name string
		args args
		want *AccountHandle
	}{
		{
			name: "test",
			args: args{
				client:      &storage.AccountsClient{},
				groupName:   "test-group",
				accountName: "test-account",
			},
			want: &AccountHandle{
				client:      &storage.AccountsClient{},
				groupName:   "test-group",
				accountName: "test-account",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAccountHandle(tt.args.client, tt.args.groupName, tt.args.accountName)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("NewAccountHandle() = %v, wantErr %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func Test(t *testing.T) {
	fileName := "/home/illya/go/src/github.com/crossplaneio/crossplane/config/creds/aks.json"
	groupName := "group-westus-1"
	accountName := "upboundquickstart4"
	ctx := context.TODO()

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatal(err)
	}

	sac, err := NewStorageAccountClient(data)
	if err != nil {
		t.Fatal(err)
	}

	ah := NewAccountHandle(sac, groupName, accountName)

	//if err := ah.IsAccountNameAvailable(ctx, accountName); err != nil {
	//	t.Fatal(err)
	//}

	//acct, err := ah.Get(ctx)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//fmt.Printf("%+v\n", *acct)
	//fmt.Printf("%+v\n", *acct.AccountProperties)

	fmt.Println("creating")
	pars := storage.AccountCreateParameters{
		Kind:     storage.Storage,
		Location: to.StringPtr("West US"),
		Sku: &storage.Sku{
			Name: storage.StandardLRS,
			Tier: storage.Standard,
		},
	}

	acct, err := ah.Create(ctx, pars)
	//_, err = sac.Create(ctx, groupName, "upboundtestacct1", pars)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%v\n", acct)
}
