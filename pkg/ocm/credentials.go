package ocm

import (
	"context"
	"fmt"
	"net/url"

	"github.com/open-component-model/ocm/pkg/common"
	"github.com/open-component-model/ocm/pkg/contexts/credentials"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigureCredentials takes a repository url and secret ref and configures access to an OCI repository.
func ConfigureCredentials(ctx context.Context, ocmCtx ocm.Context, c client.Client, repositoryURL, secretRef, namespace string) error {
	// create the consumer id for credentials
	consumerID, err := getConsumerIdentityForRepository(repositoryURL)
	if err != nil {
		return err
	}

	// fetch the credentials for the component storage
	creds, err := getCredentialsForRepository(ctx, c, namespace, secretRef)
	if err != nil {
		return err
	}

	ocmCtx.CredentialsContext().SetCredentialsForConsumer(consumerID, creds)

	return nil
}

func getConsumerIdentityForRepository(repositoryURL string) (credentials.ConsumerIdentity, error) {
	regURL, err := url.Parse(repositoryURL)
	if err != nil {
		return nil, err
	}

	if regURL.Scheme == "" {
		regURL, err = url.Parse(fmt.Sprintf("oci://%s", repositoryURL))
		if err != nil {
			return nil, err
		}
	}

	return credentials.ConsumerIdentity{
		"type":     "OCIRegistry",
		"hostname": regURL.Host,
	}, nil
}

func getCredentialsForRepository(ctx context.Context, c client.Client, namespace string, secretName string) (credentials.Credentials, error) {
	var secret corev1.Secret
	secretKey := client.ObjectKey{
		Namespace: namespace,
		Name:      secretName,
	}
	if err := c.Get(ctx, secretKey, &secret); err != nil {
		return nil, err
	}

	props := make(common.Properties)
	for key, value := range secret.Data {
		props.SetNonEmptyValue(key, string(value))
	}

	return credentials.NewCredentials(props), nil
}
