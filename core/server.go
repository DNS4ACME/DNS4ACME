package core

import (
	"context"
	"github.com/dns4acme/dns4acme/backend"
	"github.com/dns4acme/dns4acme/lang/E"
	"github.com/miekg/dns"
	"golang.org/x/sync/errgroup"
	"log"
	"log/slog"
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
		ctx:     ctx,
		config:  s.config,
		backend: s.backend,
		lock:    &sync.Mutex{},
		logger:  s.logger,
	}

	errGroup := &errgroup.Group{}
	for _, proto := range []string{"tcp", "udp"} {
		errGroup.Go(func() error {
			started := make(chan struct{})
			hasStarted := false
			var startupError error
			lock := sync.Mutex{}
			dnsServer := &dns.Server{
				Addr: s.config.Listen.String(),
				Net:  proto,
				NotifyStartedFunc: func() {
					lock.Lock()
					defer lock.Unlock()
					close(started)
				},
				MsgAcceptFunc: func(dh dns.Header) dns.MsgAcceptAction {
					return dns.MsgAccept
				},
				MsgInvalidFunc: func(m []byte, err error) {
					s.logger.Debug("Invalid DNS message", slog.String("error", err.Error()), slog.String("m", string(m)))
				},
				TsigProvider: &tsigProvider{
					s.backend,
					ctx,
				},
				Handler: srv,
			}
			go func(ctx context.Context) {
				if err := dnsServer.ListenAndServe(); err != nil {
					lock.Lock()
					defer lock.Unlock()
					if hasStarted {
						srv.onStopped(ctx, err, dnsServer)
					} else {
						startupError = err
						close(started)
					}
				}
			}(ctx)
			select {
			case <-started:
			case <-ctx.Done():
				_ = dnsServer.ShutdownContext(ctx)
				return ErrServerStartTimeout
			}

			lock.Lock()
			defer lock.Unlock()
			if startupError != nil {
				return startupError
			}
			srv.lock.Lock()
			defer srv.lock.Unlock()
			srv.dnsServers = append(srv.dnsServers, dnsServer)
			return nil
		})
	}
	if err := errGroup.Wait(); err != nil {
		srv.lock.Lock()
		defer srv.lock.Unlock()
		for _, dnsServer := range srv.dnsServers {
			_ = dnsServer.ShutdownContext(ctx)
		}
		return nil, err
	}
	return srv, nil
}

type runningServer struct {
	ctx        context.Context
	config     Config
	backend    backend.Provider
	dnsServers []*dns.Server
	lock       *sync.Mutex
	logger     *slog.Logger
}

func (r runningServer) ServeDNS(writer dns.ResponseWriter, msg *dns.Msg) {
	if msg.RecursionAvailable || msg.Authoritative || len(msg.Question) == 0 || (msg.Opcode != dns.OpcodeQuery && msg.Opcode != dns.OpcodeUpdate) {
		// These are cases that are not legitimate queries for an ACME DNS responder, so they must be either scans
		// or denial-of-service requests trying to elicit large UDP responses. While this is not RFC-conformant, we
		// will completely ignore such requests as a defense measure and only respond to legitimate queries.
		// TODO when using TCP, respond to the invalid request
		r.logger.Debug(
			"Request contains suspected DoS or scan types, ignoring...",
			slog.String("query", msg.String()),
		)
		if err := writer.Close(); err != nil {
			r.logger.Debug(
				"Cannot close connection for suspected DoS or scan request.",
				append(
					[]any{slog.String("query", msg.String())},
					E.ToSLogAttr(err)...,
				)...,
			)
		}
		return
	}
	switch msg.Opcode {
	case dns.OpcodeQuery:
		r.serveQuery(r.ctx, writer, msg)
	case dns.OpcodeUpdate:
		r.serveUpdate(r.ctx, writer, msg)
	default:
		response := &dns.Msg{}
		response.SetRcode(msg, dns.RcodeNotImplemented)
		if err := writer.WriteMsg(response); err != nil {
			r.logger.Debug(
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
	tsigStatus := writer.TsigStatus()
	response := &dns.Msg{}
	if tsigStatus != nil {
		response.SetRcode(msg, dns.RcodeNotAuth)
		if err := writer.WriteMsg(response); err != nil {
			r.logger.Debug("Cannot write response for missing signature.", E.ToSLogAttr(err)...)
		}
		return
	}
	tsig := msg.IsTsig()
	if len(msg.Question) != 1 {
		response.SetRcode(msg, dns.RcodeFormatError)
		if err := writer.WriteMsg(response); err != nil {
			r.logger.Debug("Cannot write response for missing question.", E.ToSLogAttr(err)...)
		}
		return
	}
	if msg.Question[0].Name != tsig.Hdr.Name {
		response.SetRcode(msg, dns.RcodeNotAuth)
		response.Extra = append(response.Extra, tsig)
		if err := writer.WriteMsg(response); err != nil {
			r.logger.Debug("Cannot write response for mismatching signature.", E.ToSLogAttr(err)...)
		}
		return
	}
	zone, err := r.getZone(ctx, msg.Question[0].Name)
	if err != nil {
		response.SetRcode(msg, dns.RcodeNotAuth)
		response.Extra = append(response.Extra, tsig)
		if err := writer.WriteMsg(response); err != nil {
			r.logger.Debug("Cannot write response for mismatching signature.", E.ToSLogAttr(err)...)
		}
		return
	}
	txtValues := zone.ACMEChallengeAnswers
	for _, ns := range msg.Ns {
		if ns.Header().Name != tsig.Hdr.Name {
			response.SetRcode(msg, dns.RcodeNotAuth)
			response.Extra = append(response.Extra, tsig)
			if err := writer.WriteMsg(response); err != nil {
				r.logger.Debug("Cannot write response for mismatching signature.", E.ToSLogAttr(err)...)
			}
			return
		}
		if ns.Header().Rrtype != dns.TypeTXT {
			response.SetRcode(msg, dns.RcodeRefused)
			response.Extra = append(response.Extra, tsig)
			if err := writer.WriteMsg(response); err != nil {
				r.logger.Debug("Cannot write response for non-TXT type.", E.ToSLogAttr(err)...)
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
	if err := r.backend.Set(ctx, name, txtValues); err != nil {
		response.SetRcode(msg, dns.RcodeServerFailure)
		response.Extra = append(response.Extra, tsig)
		if err := writer.WriteMsg(response); err != nil {
			r.logger.Debug("Cannot write response for backend type.", E.ToSLogAttr(err)...)
		}
		return
	}
	response.SetRcode(msg, dns.RcodeSuccess)
	response.Extra = append(response.Extra, tsig)
	if err := writer.WriteMsg(response); err != nil {
		r.logger.Debug("Cannot write update response.", E.ToSLogAttr(err)...)
	}
}

func (r runningServer) serveQuery(ctx context.Context, writer dns.ResponseWriter, msg *dns.Msg) {
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
	zoneData, err := r.getZone(ctx, question.Name)
	if err != nil {
		if E.Is(err, backend.ErrDomainNotInBackend) {
			response.SetRcode(msg, dns.RcodeRefused)
			if sig := msg.IsTsig(); sig != nil {
				response.Extra = append(response.Extra, sig)
			}
			if err = writer.WriteMsg(response); err != nil {
				r.logger.Debug("Cannot write response", E.ToSLogAttr(err)...)
			}
			return
		}
		response.SetRcode(msg, dns.RcodeServerFailure)
		if sig := msg.IsTsig(); sig != nil {
			response.Extra = append(response.Extra, sig)
		}
		if err = writer.WriteMsg(response); err != nil {
			r.logger.Debug("Cannot write response", E.ToSLogAttr(err)...)
		}
		// TODO log error
		return
	}
	response.Authoritative = true
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
	if err = writer.WriteMsg(response); err != nil {
		log.Fatal(err)
	}
}

func (r runningServer) getZone(ctx context.Context, name string) (backend.ProviderResponse, error) {
	return getZone(ctx, r.backend, name)
}

func (r runningServer) onStopped(ctx context.Context, _ error, srv *dns.Server) {
	// TODO handle/log error
	r.lock.Lock()
	defer r.lock.Unlock()
	for _, dnsServer := range r.dnsServers {
		if srv == dnsServer {
			continue
		}
		_ = dnsServer.ShutdownContext(ctx)
	}
}

func (r runningServer) Stop(ctx context.Context) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	group := &errgroup.Group{}
	for _, dnsServer := range r.dnsServers {
		group.Go(func() error {
			return dnsServer.ShutdownContext(ctx)
		})
	}
	if err := group.Wait(); err != nil {
		return ErrServerShutdownFailed.Wrap(err)
	}
	return nil
}

func getZone(ctx context.Context, backendProvider backend.Provider, name string) (backend.ProviderResponse, error) {
	if !strings.HasPrefix(name, "_acme-challenge.") {
		return backend.ProviderResponse{}, backend.ErrDomainNotInBackend
	}
	name = strings.TrimSuffix(name, ".")
	name = strings.TrimPrefix(name, "_acme-challenge.")
	zoneData, err := backendProvider.Get(ctx, name)
	return zoneData, err
}
