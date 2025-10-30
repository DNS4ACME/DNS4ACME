package core

import (
	"log/slog"
	"net/netip"

	"github.com/miekg/dns"
)

type Config struct {
	Listen *netip.AddrPort `config:"listen" default:"0.0.0.0:5353" description:"Address and port to listen on for TCP and UDP requests."`

	// Nameservers contains the list of nameservers that should be returned as part of a SOA response. This field is
	// required.
	Nameservers []string `config:"nameservers" description:"A list of nameservers to return as part of the NS and SOA responses. (required)"`

	// DebugZoneNotFound enables logging a debug message if a queried zone was not found.
	DebugZoneNotFound bool `config:"debug-zone-not-found" description:"Debug if a zone queried was not found."`
}

func (c Config) Validate() error {
	if len(c.Nameservers) == 0 {
		return ErrInvalidConfiguration.Wrap(ErrMissingNameservers)
	}
	for i, ns := range c.Nameservers {
		if ns == "" {
			return ErrInvalidConfiguration.Wrap(ErrEmptyNameserver).WithAttr(slog.Int("item", i))
		}
		if _, ok := dns.IsDomainName(ns); !ok {
			return ErrInvalidConfiguration.Wrap(ErrInvalidNameserver).WithAttr(slog.Int("item", i))
		}
	}
	return nil
}
