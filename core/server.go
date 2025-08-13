package core

import (
	"context"
	"github.com/dns4acme/dns4acme/backend"
	"github.com/dns4acme/dns4acme/lang/E"
	"github.com/miekg/dns"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

func New(config Config, backend backend.Provider, logger *slog.Logger) (Server, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if backend == nil {
		return nil, ErrMissingBackend
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &server{
		config:  config,
		backend: backend,
		logger:  logger,
	}, nil
}

type Server interface {
	Start(ctx context.Context) (RunningServer, error)
}

type RunningServer interface {
	Stop(ctx context.Context) error
}

type server struct {
	config  Config
	backend backend.Provider
	logger  *slog.Logger
}

func (s server) Start(ctx context.Context) (RunningServer, error) {
	srv := &runningServer{
		ctx:               ctx,
		config:            s.config,
		backend:           s.backend,
		logger:            s.logger,
		dnsServersRunning: map[*dns.Server]bool{},
		dnsServersClose:   map[*dns.Server]chan struct{}{},
		dnsServerLocks:    map[*dns.Server]*sync.Mutex{},
	}
	for _, proto := range []string{"tcp", "udp"} {
		s.logger.DebugContext(
			ctx,
			"Starting DNS4ACME listener...",
			slog.String("proto", proto),
			slog.String("address", s.config.Listen.String()),
		)
		started := make(chan struct{})
		hasStarted := false
		var startupError error
		var dnsServer *dns.Server
		dnsServer = &dns.Server{
			Addr: s.config.Listen.String(),
			Net:  proto,
			MsgAcceptFunc: func(dh dns.Header) dns.MsgAcceptAction {
				if isResponse := dh.Bits&(1<<15) != 0; isResponse {
					return dns.MsgIgnore
				}
				opcode := int(dh.Bits>>11) & 0xF
				if opcode != dns.OpcodeQuery && opcode != dns.OpcodeNotify && opcode != dns.OpcodeUpdate {
					return dns.MsgRejectNotImplemented
				}
				return dns.MsgAccept
			},
			NotifyStartedFunc: func() {
				srv.dnsServerLocks[dnsServer].Lock()
				defer srv.dnsServerLocks[dnsServer].Unlock()
				hasStarted = true
				close(started)
			},
			MsgInvalidFunc: func(_ []byte, err error) {
				s.logger.DebugContext(ctx, "Invalid DNS message", E.ToSLogAttr(err)...)
			},
			TsigProvider: &tsigProvider{
				s.logger,
				s.backend,
				ctx,
			},
			Handler: srv,
		}
		srv.dnsServers = append(srv.dnsServers, dnsServer)
		srv.dnsServersRunning[dnsServer] = false
		srv.dnsServersClose[dnsServer] = make(chan struct{})
		srv.dnsServerLocks[dnsServer] = &sync.Mutex{}
		go func(ctx context.Context) {
			err := dnsServer.ListenAndServe()
			if hasStarted {
				srv.onStopped(ctx, err, dnsServer)
			} else {
				srv.dnsServerLocks[dnsServer].Lock()
				defer srv.dnsServerLocks[dnsServer].Unlock()
				startupError = err
				close(started)
			}
			close(srv.dnsServersClose[dnsServer])
		}(ctx)

		select {
		case <-started:
		case <-ctx.Done():
			_ = srv.Stop(ctx)
			return nil, ErrServerStartTimeout
		}

		if startupError != nil {
			_ = srv.Stop(ctx)
			return nil, startupError
		}
		srv.dnsServerLocks[dnsServer].Lock()
		srv.dnsServersRunning[dnsServer] = true
		srv.dnsServerLocks[dnsServer].Unlock()
		s.logger.DebugContext(
			ctx,
			"DNS4ACME listener running",
			slog.String("proto", proto),
			slog.String("address", s.config.Listen.String()),
		)
	}
	s.logger.InfoContext(ctx, "DNS4ACME running", slog.String("listen", srv.config.Listen.String()))
	return srv, nil
}

type runningServer struct {
	ctx               context.Context
	config            Config
	backend           backend.Provider
	dnsServers        []*dns.Server
	dnsServersRunning map[*dns.Server]bool
	dnsServersClose   map[*dns.Server]chan struct{}
	dnsServerLocks    map[*dns.Server]*sync.Mutex
	logger            *slog.Logger
}

func (r runningServer) ServeDNS(writer dns.ResponseWriter, msg *dns.Msg) {
	switch msg.Opcode {
	case dns.OpcodeQuery:
		r.serveQuery(r.ctx, writer, msg)
	case dns.OpcodeUpdate:
		r.serveUpdate(r.ctx, writer, msg)
	default:
		response := &dns.Msg{}
		response.SetRcode(msg, dns.RcodeNotImplemented)
		if err := writer.WriteMsg(response); err != nil {
			r.logger.DebugContext(
				r.ctx,
				"Cannot write response to unsupported query.",
				append(
					[]any{slog.String("query", msg.String())},
					E.ToSLogAttr(err)...,
				)...,
			)
		}
	}
}

func (r runningServer) serveUpdate(ctx context.Context, writer dns.ResponseWriter, msg *dns.Msg) {
	logger := r.logger.With(slog.String("remote", writer.RemoteAddr().String()), slog.String("local", writer.LocalAddr().String()))
	response := &dns.Msg{}
	if len(msg.Question) != 1 {
		response.SetRcode(msg, dns.RcodeFormatError)
		if err := writer.WriteMsg(response); err != nil {
			logger.DebugContext(ctx, "Cannot write response for missing question.", E.ToSLogAttr(err)...)
		}
		return
	}
	tsigStatus := writer.TsigStatus()
	if tsigStatus != nil {
		if zone, err := r.getZone(ctx, msg.Question[0].Name); err == nil && zone.Debug {
			logger.DebugContext(ctx, "Cannot update zone", E.ToSLogAttr(tsigStatus, slog.String("zone", msg.Question[0].Name))...)
		}
		response.SetRcode(msg, dns.RcodeNotAuth)
		if err := writer.WriteMsg(response); err != nil {
			logger.DebugContext(ctx, "Cannot write response for missing signature.", E.ToSLogAttr(err)...)
		}
		return
	}
	tsig := msg.IsTsig()
	if tsig == nil {
		if zone, err := r.getZone(ctx, msg.Question[0].Name); err == nil && zone.Debug {
			logger.DebugContext(ctx, "Cannot update zone", slog.String("zone", msg.Question[0].Name), slog.String("error_message", "TSIG missing"))
		}
		response.SetRcode(msg, dns.RcodeNotAuth)
		if err := writer.WriteMsg(response); err != nil {
			logger.DebugContext(ctx, "Cannot write response for missing signature.", E.ToSLogAttr(err)...)
		}
		return
	}
	zone, err := r.getZone(ctx, msg.Question[0].Name)
	if err != nil {
		response.SetRcode(msg, dns.RcodeNotAuth)
		response.Extra = append(response.Extra, tsig)
		if err := writer.WriteMsg(response); err != nil {
			logger.DebugContext(ctx, "Cannot write response for mismatching signature.", E.ToSLogAttr(err)...)
		}
		return
	}
	logger = logger.With(slog.String("zone", msg.Question[0].Name))

	key, err := r.backend.GetKey(ctx, strings.TrimSuffix(tsig.Hdr.Name, "."))
	if err != nil {
		if zone.Debug {
			logger.DebugContext(ctx, "Cannot update zone", E.ToSLogAttr(err, slog.String("key", tsig.Hdr.Name))...)
		}

		response.SetRcode(msg, dns.RcodeNotAuth)
		response.Extra = append(response.Extra, tsig)
		if err := writer.WriteMsg(response); err != nil {
			logger.DebugContext(ctx, "Cannot write response for missing key.", E.ToSLogAttr(err)...)
		}
		return
	}
	logger = logger.With(slog.String("key", tsig.Hdr.Name))
	if !slices.Contains(key.Zones, strings.TrimPrefix(strings.TrimSuffix(msg.Question[0].Name, "."), "_acme-challenge.")) {
		if zone.Debug {
			logger.DebugContext(ctx, "Cannot update zone", slog.String("error_message", "Key is not authorized to modify zone"))
		}
		response.SetRcode(msg, dns.RcodeNotAuth)
		response.Extra = append(response.Extra, tsig)
		if err := writer.WriteMsg(response); err != nil {
			logger.DebugContext(ctx, "Cannot write response for missing permissions.", E.ToSLogAttr(err)...)
		}
		return
	}
	txtValues := zone.ACMEChallengeAnswers
	for _, ns := range msg.Ns {
		if ns.Header().Name != msg.Question[0].Name {
			if zone.Debug {
				logger.DebugContext(ctx, "Cannot update zone", slog.String("error_message", "Attempting to update out-of-zone record"), slog.String("record_name", ns.Header().Name))
			}
			response.SetRcode(msg, dns.RcodeNotAuth)
			response.Extra = append(response.Extra, tsig)
			if err := writer.WriteMsg(response); err != nil {
				logger.DebugContext(ctx, "Cannot write response for mismatching signature.", E.ToSLogAttr(err)...)
			}
			return
		}
		if ns.Header().Rrtype != dns.TypeTXT {
			logger.DebugContext(ctx, "Cannot update zone", slog.String("error_message", "Attempting to update non-TXT record"), slog.String("record_type", dns.TypeToString[ns.Header().Rrtype]))
			response.SetRcode(msg, dns.RcodeRefused)
			response.Extra = append(response.Extra, tsig)
			if err := writer.WriteMsg(response); err != nil {
				logger.DebugContext(ctx, "Cannot write response for non-TXT type.", E.ToSLogAttr(err)...)
			}
			return
		}
		txt := ns.(*dns.TXT)
		if txt.Txt == nil {
			txtValues = txtValues[:0]
		} else {
			txtValues = append(txtValues, strings.Join(txt.Txt, ""))
		}
	}
	name := msg.Question[0].Name
	name = strings.TrimSuffix(name, ".")
	name = strings.TrimPrefix(name, "_acme-challenge.")
	if err := r.backend.SetZone(ctx, name, txtValues); err != nil {
		logger.DebugContext(ctx, "Cannot update zone", E.ToSLogAttr(err)...)
		response.SetRcode(msg, dns.RcodeServerFailure)
		response.Extra = append(response.Extra, tsig)
		if err := writer.WriteMsg(response); err != nil {
			logger.DebugContext(ctx, "Cannot write response for backend type.", E.ToSLogAttr(err)...)
		}
		return
	}
	response.SetRcode(msg, dns.RcodeSuccess)
	response.Extra = append(response.Extra, tsig)
	if err := writer.WriteMsg(response); err != nil {
		logger.DebugContext(ctx, "Cannot write update response.", E.ToSLogAttr(err)...)
	}
}

func (r runningServer) serveQuery(ctx context.Context, writer dns.ResponseWriter, msg *dns.Msg) {
	logger := r.logger.With(slog.String("remote", writer.RemoteAddr().String()), slog.String("local", writer.LocalAddr().String()))
	response := &dns.Msg{}

	if len(msg.Question) != 1 {
		response.SetRcode(msg, dns.RcodeFormatError)
		if sig := msg.IsTsig(); sig != nil {
			response.Extra = append(response.Extra, sig)
		}
		_ = writer.WriteMsg(response)
		return
	}
	question := msg.Question[0]
	logger = logger.With(slog.String("zone", question.Name))
	zoneData, err := r.getZone(ctx, question.Name)
	if err != nil {
		if E.Is(err, backend.ErrZoneNotInBackend) {
			response.SetRcode(msg, dns.RcodeRefused)
			if sig := msg.IsTsig(); sig != nil {
				response.Extra = append(response.Extra, sig)
			}
			if err = writer.WriteMsg(response); err != nil {
				logger.DebugContext(ctx, "Cannot write response", E.ToSLogAttr(err)...)
			}
			return
		}
		response.SetRcode(msg, dns.RcodeServerFailure)
		if sig := msg.IsTsig(); sig != nil {
			response.Extra = append(response.Extra, sig)
		}
		if err = writer.WriteMsg(response); err != nil {
			logger.DebugContext(ctx, "Cannot write response", E.ToSLogAttr(err)...)
		}
		return
	}
	response.Authoritative = true
	if zoneData.Debug {
		logger.DebugContext(ctx, "Query: ", question.String())
	}

	switch question.Qtype {
	case dns.TypeTXT:
		response.SetRcode(msg, dns.RcodeSuccess)
		response.Answer = []dns.RR{}
		for _, txt := range zoneData.ACMEChallengeAnswers {
			var txtData []string
			for len(txt) > 0 {
				l := min(len(txt), 255)
				record := txt[:l]
				txtData = append(txtData, record)
				txt = txt[l:]
			}
			response.Answer = append(response.Answer, &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				Txt: txtData,
			})
		}
	case dns.TypeSOA:
		response.SetRcode(msg, dns.RcodeSuccess)
		response.Answer = []dns.RR{
			&dns.SOA{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    86400,
				},
				Ns:      r.config.Nameservers[0] + ".",
				Mbox:    "nomail." + r.config.Nameservers[0] + ".",
				Serial:  zoneData.Serial,
				Refresh: 86400,
				Retry:   7200,
				Expire:  3600000,
				Minttl:  60,
			},
		}
	case dns.TypeNS:
		response.SetRcode(msg, dns.RcodeSuccess)
		response.Answer = make([]dns.RR, len(r.config.Nameservers))
		for i, ns := range r.config.Nameservers {
			response.Answer[i] = &dns.NS{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    86400,
				},
				Ns: ns + ".",
			}
		}
	default:
		response.SetRcode(msg, dns.RcodeSuccess)
	}
	if sig := msg.IsTsig(); sig != nil {
		response.Extra = append(response.Extra, sig)
	}
	if zoneData.Debug {
		logger.DebugContext(ctx, "Response: ", response.String())
	}
	if err = writer.WriteMsg(response); err != nil {
		logger.DebugContext(ctx, "Error writing response", E.ToSLogAttr(err)...)
	}
}

func (r runningServer) getZone(ctx context.Context, name string) (backend.ProviderZoneResponse, error) {
	return getZone(ctx, r.backend, name)
}

func (r runningServer) onStopped(ctx context.Context, err error, srv *dns.Server) {
	r.dnsServerLocks[srv].Lock()
	// No locking needed, the caller already locks
	args := []any{
		slog.String("proto", srv.Net),
		slog.String("address", srv.Addr),
	}
	if err != nil {
		args = append(args, E.ToSLogAttr(err)...)
	}
	r.logger.DebugContext(ctx, "DNS4ACME listener stopped", args...)
	r.dnsServersRunning[srv] = false
	r.dnsServerLocks[srv].Unlock()

	for _, dnsServer := range r.dnsServers {
		if srv == dnsServer {
			continue
		}
		r.logger.DebugContext(
			ctx,
			"Shutting down listener",
			slog.String("proto", dnsServer.Net),
			slog.String("address", dnsServer.Addr),
		)
		if err := r.shutdownListener(ctx, dnsServer); err != nil {
			r.logger.WarnContext(
				ctx,
				"Error shutting down listener",
				E.ToSLogAttr(err)...,
			)
		}
	}
}

func (r runningServer) Stop(ctx context.Context) error {
	r.logger.InfoContext(ctx, "Stopping DNS4ACME...")
	for _, dnsServer := range r.dnsServers {
		if err := r.shutdownListener(ctx, dnsServer); err != nil {
			r.logger.ErrorContext(ctx, "DNS4ACME shutdown failed", E.ToSLogAttr(err)...)
			return ErrServerShutdownFailed.Wrap(err)
		}
	}
	r.logger.InfoContext(ctx, "DNS4ACME shutdown complete, no errors.")
	return nil
}

func getZone(ctx context.Context, backendProvider backend.Provider, name string) (backend.ProviderZoneResponse, error) {
	if !strings.HasPrefix(name, "_acme-challenge.") {
		return backend.ProviderZoneResponse{}, backend.ErrZoneNotInBackend
	}
	name = strings.TrimSuffix(name, ".")
	name = strings.TrimPrefix(name, "_acme-challenge.")
	zoneData, err := backendProvider.GetZone(ctx, name)
	return zoneData, err
}

func (r runningServer) shutdownListener(ctx context.Context, dnsServer *dns.Server) error {
	r.dnsServerLocks[dnsServer].Lock()
	if !r.dnsServersRunning[dnsServer] {
		r.dnsServerLocks[dnsServer].Unlock()
		return nil
	}
	r.logger.DebugContext(
		ctx,
		"Shutting down listener",
		slog.String("proto", dnsServer.Net),
		slog.String("address", dnsServer.Addr),
	)
	err := dnsServer.ShutdownContext(ctx)
	if err != nil {
		r.dnsServersRunning[dnsServer] = false
	}
	r.dnsServerLocks[dnsServer].Unlock()
	if err != nil {
		return ErrListenerShutdownFailed.
			Wrap(err).
			WithAttr(slog.String("proto", dnsServer.Net)).
			WithAttr(slog.String("address", dnsServer.Addr))
	}
	select {
	case <-ctx.Done():
		return ErrListenerShutdownTimeout.Wrap(ctx.Err())
	case <-r.dnsServersClose[dnsServer]:
	}

	return nil
}
