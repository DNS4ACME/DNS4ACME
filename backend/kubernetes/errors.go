package kubernetes

import "github.com/dns4acme/dns4acme/lang/E"

var ErrCRDMissing = E.New("KUBERNETES_CRD_MISSING", "the CRD is missing in the Kubernetes cluster")
