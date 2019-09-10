package utils

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	awsv1alpha1 "github.com/crossplaneio/crossplane/aws/apis/v1alpha1"
	awsclients "github.com/crossplaneio/crossplane/pkg/clients/aws"
)

// RetrieveAwsConfigFromProvider retrieves the aws config from the given aws provider reference
func RetrieveAwsConfigFromProvider(ctx context.Context, client client.Reader, providerRef *corev1.ObjectReference) (*aws.Config, error) {
	p := &awsv1alpha1.Provider{}
	n := meta.NamespacedNameOf(providerRef)
	if err := client.Get(ctx, n, p); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider %s", n)
	}

	secret := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.GetNamespace(), Name: p.Spec.Secret.Name}
	err := client.Get(ctx, n, secret)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get provider secret %s", n)
	}

	cfg, err := awsclients.LoadConfig(secret.Data[p.Spec.Secret.Key], awsclients.DefaultSection, p.Spec.Region)

	return cfg, errors.Wrap(err, "cannot create new AWS configuration")
}
