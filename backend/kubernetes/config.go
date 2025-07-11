package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/dns4acme/dns4acme/backend"
	"github.com/dns4acme/dns4acme/lang/E"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Host     string `config:"host" default:"kubernetes.default.svc" description:"Host name for the Kubernetes cluster API server."`
	APIPath  string `config:"path" default:"/api" description:"Path for the API endpoint."`
	BasePath string `config:"base_path" default:"/" description:"Base path for the REST API endpoint."`

	Namespace string `config:"namespace" default:"default" description:"Namespace to look for DNS4ACME resources in."`

	Username string `config:"username" description:"Username for authenticating to the Kubernetes API."`
	Password string `config:"password" description:"Password for authenticating to the Kubernetes API."`

	ServerName string `config:"server-name" description:"SNI name to pass to the Kubernetes API server."`

	CertData []byte `config:"cert" description:"PEM-encoded client certificate to use for authenticating to the Kubernetes API."`
	KeyData  []byte `config:"key" description:"PEM-encoded client private key to use for authenticating to the Kubernetes API."`
	CAData   []byte `config:"cacert" description:"PEM-encoded Certificate Authority to verify the connection to the Kubernetes API."`

	CertFile string `config:"cert-file" description:"File containing the PEM-encoded client certificate to use for authenticating to the Kubernetes API. Set to /var/run/secrets/kubernetes.io/serviceaccount/ca.crt for in-cluster operation."`
	KeyFile  string `config:"key-file" description:"File containing the PEM-encoded client private key to use for authenticating to the Kubernetes API."`
	CAFile   string `config:"cacert-file" description:"File containing the PEM-encoded CA certificate to verify the connection to the Kubernetes API."`

	BearerToken     string `config:"bearer-token" description:"Token used to authenticate to the Kubernetes API."`
	BearerTokenFile string `config:"bearer-token-file" description:"File containing the bearer token used to authenticate to the Kubernetes API. Set to /var/run/secrets/kubernetes.io/serviceaccount/token for in-cluster authentication."`

	QPS     float32       `config:"qps" default:"5" description:"Maximum QPS to use for Kubernetes API requests."`
	Burst   int           `config:"burst" default:"10" description:"Maximum burst to use for Kubernetes API requests."`
	Timeout time.Duration `config:"timeout" default:"5s" description:"Maximum time to wait for a response from the Kubernetes API."`

	Logger *slog.Logger `json:"-"`
}

func (c Config) Build(ctx context.Context) (backend.Provider, error) {
	return c.BuildFull(ctx)
}

func (c Config) BuildExtended(ctx context.Context) (backend.ExtendedProvider, error) {
	return c.BuildFull(ctx)
}

func decodeCertData(c []byte) ([]byte, error) {
	if len(c) == 0 {
		return nil, nil
	}
	if c[0] == '-' {
		// The CertData doesn't need additional base64 decoding as it is already in PEM format.
		return c, nil
	}
	data := make([]byte, base64.StdEncoding.DecodedLen(len(c)))
	_, err := base64.StdEncoding.Decode(data, c)
	return data, err
}

func (c Config) BuildFull(ctx context.Context) (Provider, error) {
	certData, err := decodeCertData(c.CertData)
	if err != nil {
		return nil, backend.ErrConfiguration.Wrap(fmt.Errorf("failed to decode certificate data: %w", err))
	}
	keyData, err := decodeCertData(c.KeyData)
	if err != nil {
		return nil, backend.ErrConfiguration.Wrap(fmt.Errorf("failed to decode key data: %w", err))
	}
	caData, err := decodeCertData(c.CAData)
	if err != nil {
		return nil, backend.ErrConfiguration.Wrap(fmt.Errorf("failed to decode CA cert data: %w", err))
	}
	cfg := restclient.Config{
		Host:            c.Host,
		APIPath:         "/" + strings.Trim(strings.Trim(c.BasePath, "/")+"/apis", "/"),
		Username:        c.Username,
		Password:        c.Password,
		BearerToken:     c.BearerToken,
		BearerTokenFile: c.BearerTokenFile,
		Impersonate:     restclient.ImpersonationConfig{},
		TLSClientConfig: restclient.TLSClientConfig{
			ServerName: c.ServerName,
			CertData:   certData,
			KeyData:    keyData,
			CAData:     caData,
			CertFile:   c.CertFile,
			KeyFile:    c.KeyFile,
			CAFile:     c.CAFile,
		},
		UserAgent: "DNS4ACME",
		QPS:       c.QPS,
		Burst:     c.Burst,
		Timeout:   c.Timeout,
	}
	logger := c.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	logger = logger.WithGroup("kubernetes")
	p := &provider{
		config:           c,
		zones:            nil,
		keys:             nil,
		keyBindings:      nil,
		secrets:          nil,
		logger:           logger,
		dynamicClient:    nil,
		keyBindingsLock:  &sync.RWMutex{},
		keyBindingsByKey: map[string]map[string]keyBindingSpec{},
	}
	cfg.WarningHandlerWithContext = p
	p.logger.DebugContext(ctx, "Starting Kubernetes monitoring...")
	p.dynamicClient, err = dynamic.NewForConfig(&cfg)
	if err != nil {
		return nil, backend.ErrConfiguration.Wrap(err)
	}
	p.zones, err = newObjectCRUD[*zone](ctx, p.dynamicClient, c.Namespace, zoneKind, zoneGroupVersionResource, logger, nil)
	if err != nil {
		if E.Is(err, backend.ErrObjectNotInBackend) {
			err = ErrCRDMissing.Wrap(err)
		}
		p.logger.ErrorContext(ctx, err.Error(), E.ToSLogAttr(err)...)
		return nil, err
	}
	p.keys, err = newObjectCRUD[*key](ctx, p.dynamicClient, c.Namespace, keyKind, keyGroupVersionResource, logger, nil)
	if err != nil {
		if E.Is(err, backend.ErrObjectNotInBackend) {
			err = ErrCRDMissing.Wrap(err)
		}
		p.logger.ErrorContext(ctx, err.Error(), E.ToSLogAttr(err)...)
		return nil, err
	}
	p.keyBindings, err = newObjectCRUD[*keyBinding](ctx, p.dynamicClient, c.Namespace, keyBindingKind, keyBindingGroupVersionResource, logger, p.updateKeyBindingIndex)
	if err != nil {
		if E.Is(err, backend.ErrObjectNotInBackend) {
			err = ErrCRDMissing.Wrap(err)
		}
		p.logger.ErrorContext(ctx, err.Error(), E.ToSLogAttr(err)...)
		return nil, err
	}
	p.secrets, err = newObjectCRUD[*secret](ctx, p.dynamicClient, c.Namespace, secretKind, secretGroupVersionResource, logger, nil)
	if err != nil {
		if E.Is(err, backend.ErrObjectNotInBackend) {
			err = ErrCRDMissing.Wrap(err)
		}
		p.logger.ErrorContext(ctx, err.Error(), E.ToSLogAttr(err)...)
		return nil, err
	}

	return p, nil
}
