/*
 Copyright 2022 The Crossplane Authors.

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

package connection

import (
	"context"
	"crypto/tls"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store/kubernetes"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store/plugin"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errFmtUnknownSecretStore = "unknown secret store type: %q"
)

// RuntimeStoreBuilder builds and returns a Store for any supported Store type
// in a given config.
//
// All in-tree connection Store implementations needs to be registered here.
func RuntimeStoreBuilder(ctx context.Context, local client.Client, tcfg *tls.Config, cfg v1.SecretStoreConfig) (Store, error) {
	switch *cfg.Type {
	case v1.SecretStoreKubernetes:
		return kubernetes.NewSecretStore(ctx, local, nil, cfg)
	case v1.SecretStorePlugin:
		return plugin.NewSecretStore(ctx, local, tcfg, cfg)
	}
	return nil, errors.Errorf(errFmtUnknownSecretStore, *cfg.Type)
}
