package kubernetes_test

import (
	"context"
	"github.com/dns4acme/dns4acme/backend/kubernetes"
	"github.com/dns4acme/dns4acme/internal/testlogger"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"slices"
	"testing"
	"time"
)

func TestProvider(t *testing.T) {
	logger := testlogger.New(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Could not determine user home directory, skipping test (%v)", err)
	}
	kubeconfigPath := home + "/.kube/config"
	clientConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		nil).ClientConfig()
	if err != nil {
		t.Skipf("Could not load user kubeconfig from %s, skipping test (%v)", kubeconfigPath, err)
	}

	config := kubernetes.Config{
		// TODO this namespace must exist for the test to work.
		Namespace:       "dns4acme",
		Host:            clientConfig.Host,
		APIPath:         clientConfig.APIPath,
		Username:        clientConfig.Username,
		Password:        clientConfig.Password,
		ServerName:      clientConfig.ServerName,
		CertData:        clientConfig.CertData,
		KeyData:         clientConfig.KeyData,
		CAData:          clientConfig.CAData,
		CertFile:        clientConfig.CertFile,
		KeyFile:         clientConfig.KeyFile,
		CAFile:          clientConfig.CAFile,
		BearerToken:     clientConfig.BearerToken,
		BearerTokenFile: clientConfig.BearerTokenFile,
		QPS:             clientConfig.QPS,
		Burst:           clientConfig.Burst,
		Timeout:         clientConfig.Timeout,
		Logger:          logger,
	}
	if config.APIPath == "" {
		config.APIPath = "/api"
	}

	provider, err := config.BuildFull(context.Background())
	if err != nil {
		t.Skipf("Cannot build Kubernetes provider from config, skipping test (%v)", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := provider.Close(ctx); err != nil {
			t.Errorf("Failed to close provider: %v", err)
		}
	})

	// TODO the CRD must be deployed for this to work.

	if err := provider.Create(t.Context(), "test.example.com", "asdf"); err != nil {
		t.Fatalf("Failed to create test domain: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := provider.Delete(ctx, "test.example.com"); err != nil {
			t.Errorf("Failed to delete test domain: %v", err)
		}
	})
	initialData, err := provider.Get(t.Context(), "test.example.com")
	if err != nil {
		t.Fatalf("Failed to get initial data: %v", err)
	}
	if err := provider.Set(t.Context(), "test.example.com", []string{"Hello world!"}); err != nil {
		t.Fatalf("Failed to set test domain: %v", err)
	}
	nextData, err := provider.Get(t.Context(), "test.example.com")
	if err != nil {
		t.Errorf("Failed to get next data: %v", err)
	}
	if nextData.Serial <= initialData.Serial {
		t.Fatalf("Backend did not increment the serial.")
	}
	if !slices.Equal(nextData.ACMEChallengeAnswers, []string{"Hello world!"}) {
		t.Fatalf("Incorrect ACME challenge answers returned: %v", nextData.ACMEChallengeAnswers)
	}
}
