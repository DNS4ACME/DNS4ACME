package core

import (
	"github.com/dns4acme/dns4acme/lang/E"
)

var ErrInvalidConfiguration = E.New("INVALID_CONFIGURATION", "invalid configuration")
var ErrMissingNameservers = E.New("MISSING_NAMESERVERS", "nameservers are required for NS delegation")
var ErrEmptyNameserver = E.New("EMPTY_NAMESERVER", "empty nameserver encountered")
var ErrInvalidNameserver = E.New("INVALID_NAMESERVER", "invalid nameserver encountered")
var ErrMissingBackend = E.New("MISSING_BACKEND", "backend missing")
var ErrServerStartTimeout = E.New("SERVER_START_TIMEOUT", "timeout while trying to start DNS server")
var ErrServerShutdownFailed = E.New("SERVER_SHUTDOWN_FAILED", "server shutdown failed")
var ErrInvalidTsigKey = E.New("INVALID_TSIG_KEY", "invalid TSIG key")
var ErrUnsupportedTsigAlgorithm = E.New("UNSUPPORTED_TSIG_ALGORITHM", "unsupported TSIG algorithm")
var ErrListenerShutdownFailed = E.New("LISTENER_SHUTDOWN_FAILED", "listener shutdown failed")
var ErrListenerShutdownTimeout = E.New("SHUTDOWN_TIMEOUT", "timeout during shutdown")
