package dns4acme

import (
	"context"
	"fmt"
	"github.com/dns4acme/dns4acme/core"
	"io"
	"log/slog"
)

func New(ctx context.Context, config *Config, output io.Writer) (core.Server, error) {
	if _, ok := config.BackendConfigs[config.Backend]; !ok {
		return nil, core.ErrInvalidConfiguration.Wrap(fmt.Errorf("backend %s does not exist", config.Backend))
	}
	backendProvider, err := config.BackendConfigs[config.Backend].Build(ctx)
	if err != nil {
		// TODO better error handling
		return nil, err
	}

	logger := slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{
		AddSource:   false,
		Level:       config.Log.Level,
		ReplaceAttr: nil,
	}))

	srv, err := core.New(config.Config, backendProvider, logger)
	if err != nil {
		return nil, err
	}
	return srv, nil
}
