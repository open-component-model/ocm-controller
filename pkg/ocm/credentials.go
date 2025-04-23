package ocm

import (
	"context"
	"fmt"
	"net/url"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"ocm.software/ocm/api/credentials"
	credconfig "ocm.software/ocm/api/credentials/config"
	"ocm.software/ocm/api/credentials/extensions/repositories/dockerconfig"
	"ocm.software/ocm/api/ocm"
	common "ocm.software/ocm/api/utils/misc"

	"ocm.software/ocm/api/utils/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigureCredentials takes a repository url and secret ref and configures access to an OCI repository.
func ConfigureCredentials(ctx context.Context, ocmCtx ocm.Context, c client.Client, repositoryURL, secretRef, namespace string) error {
	var secret corev1.Secret
	secretKey := client.ObjectKey{
		Namespace: namespace,
		Name:      secretRef,
	}
	if err := c.Get(ctx, secretKey, &secret); err != nil {
		return fmt.Errorf("failed to locate secret: %w", err)
	}

	if dockerConfigBytes, ok := secret.Data[corev1.DockerConfigJsonKey]; ok {
		if err := configureDockerConfigCredentials(ocmCtx, dockerConfigBytes); err != nil {
			return err
		}

		return nil
	}

	if ocmConfigBytes, ok := secret.Data[v1alpha1.OCMCredentialConfigKey]; ok {
		if err := configureOcmConfigCredentials(ocmCtx, ocmConfigBytes, secret); err != nil {
			return err
		}

		return nil
	}

	// proceed to configure a credential context using username and password

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

func configureOcmConfigCredentials(ocmCtx ocm.Context, ocmConfigBytes []byte, secret corev1.Secret) error {
	cfg, err := ocmCtx.ConfigContext().GetConfigForData(ocmConfigBytes, runtime.DefaultYAMLEncoding)
	if err != nil {
		return err
	}

	if cfg.GetKind() == credconfig.ConfigType {
		if err := ocmCtx.ConfigContext().ApplyConfig(cfg, fmt.Sprintf("ocm config secret: %s/%s", secret.Namespace, secret.Name)); err != nil {
			return err
		}
	}

	return nil
}

func configureDockerConfigCredentials(ocmCtx ocm.Context, dockerConfigBytes []byte) error {
	spec := dockerconfig.NewRepositorySpecForConfig(dockerConfigBytes, true)

	if _, err := ocmCtx.CredentialsContext().RepositoryForSpec(spec); err != nil {
		return fmt.Errorf("cannot create credentials from secret: %w", err)
	}

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
