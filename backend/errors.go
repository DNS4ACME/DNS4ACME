package backend

import (
	"github.com/dns4acme/dns4acme/lang/E"
)

var ErrDomainNotInBackend = E.New("DOMAIN_NOT_IN_BACKEND", "domain not found in backend")
var ErrBackendRequestFailed = E.New("BACKEND_REQUEST_FAILED", "backend request failed")
var ErrConfiguration = E.New("CONFIGURATION_ERROR", "configuration error")
