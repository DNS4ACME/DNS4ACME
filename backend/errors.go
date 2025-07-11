package backend

import (
	"github.com/dns4acme/dns4acme/lang/E"
)

var ErrKeyNotFoundInBackend = E.New("KEY_NOT_IN_BACKEND", "key not found in backend")
var ErrBackendRequestFailed = E.New("BACKEND_REQUEST_FAILED", "backend request failed")
var ErrConfiguration = E.New("CONFIGURATION_ERROR", "configuration error")

var ErrZoneNotInBackend = E.New("ZONE_NOT_IN_BACKEND", "zone not found in backend")
var ErrZoneAlreadyExistsInBackend = E.New("ZONE_ALREADY_EXISTS", "zone already exists in backend")

var ErrObjectNotInBackend = E.New("OBJECT_NOT_IN_BACKEND", "object not found in backend")
var ErrObjectBackendConflict = E.New("OBJECT_CONFLICT", "object conflict exists in backend")
