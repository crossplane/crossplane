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
	"log"

	"google.golang.org/api/option"

	"golang.org/x/oauth2/google"

	"cloud.google.com/go/storage"
)

func NewStorageClient(ctx context.Context, creds *google.Credentials) (*storage.Client, error) {
	client, err := storage.NewClient(ctx, option.WithCredentials(creds))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	b := client.Bucket("foo")
	b.Create()

}
