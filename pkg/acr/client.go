package acr

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/kubelet/pkg/apis/credentialprovider/v1beta1"
)

type Client struct{}

type Credentials struct {
	UserName   string
	Password   string
	ExpireTime time.Time
}

func (c *Client) GetCredentials(ctx context.Context, image string, args []string) (response *v1.CredentialProviderResponse, err error) {
	registry, err := parseServerURL(image)
	if err != nil {
		return nil, err
	}
	var creds *Credentials
	if registry.IsEE {
		client, err := newEEClient(registry.Region)
		if err != nil {
			return nil, err
		}
		if registry.InstanceId == "" {
			instanceId, err := client.getInstanceId(registry.InstanceName)
			if err != nil {
				return nil, err
			}
			registry.InstanceId = instanceId
		}
		creds, err = client.getCredentials(registry.InstanceId)
		if err != nil {
			return nil, err
		}
	} else {
		client, err := newPersonClient(registry.Region)
		if err != nil {
			return nil, err
		}
		creds, err = client.getCredentials()
		if err != nil {
			return nil, err
		}
	}
	cacheDuration := getCacheDuration(&creds.ExpireTime)
	return &v1.CredentialProviderResponse{
		CacheKeyType:  v1.RegistryPluginCacheKeyType,
		CacheDuration: cacheDuration,
		Auth: map[string]v1.AuthConfig{
			registry.Domain: {
				Username: creds.UserName,
				Password: creds.Password,
			},
		},
	}, nil
}

// getCacheDuration calculates the credentials cache duration based on the ExpiresAt time from the authorization data
func getCacheDuration(expiresAt *time.Time) *metav1.Duration {
	var cacheDuration *metav1.Duration
	if expiresAt == nil {
		// explicitly set cache duration to 0 if expiresAt was nil so that
		// kubelet does not cache it in-memory
		cacheDuration = &metav1.Duration{Duration: 0}
	} else {
		// halving duration in order to compensate for the time loss between
		// the token creation and passing it all the way to kubelet.
		duration := time.Second * time.Duration((expiresAt.Unix()-time.Now().Unix())/2)
		if duration > 0 {
			cacheDuration = &metav1.Duration{Duration: duration}
		}
	}
	return cacheDuration
}
