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
