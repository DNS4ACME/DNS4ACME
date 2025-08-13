package dns4acme_test

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/dns4acme/dns4acme"
	"github.com/dns4acme/dns4acme/backend"
	"github.com/dns4acme/dns4acme/backend/inmemory"
	"github.com/dns4acme/dns4acme/internal/testlogger"
	"github.com/miekg/dns"
	"math/rand"
	"net/netip"
	"testing"
	"time"
)

func TestIntegration(t *testing.T) {
	cfg := dns4acme.NewConfig()
	addrPort := netip.AddrPortFrom(
		netip.MustParseAddr("127.0.0.1"),
		30053,
	)

	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	updateKeyData := make([]byte, 32)
	for i := 0; i < 32; i++ {
		updateKeyData[i] = byte(letters[rand.Intn(len(letters))]) //nolint:gosec // This is a test and doesn't need strong randomness
	}
	updateSecret := base64.StdEncoding.EncodeToString(updateKeyData)
	invalidUpdateKeyData := make([]byte, 32)
	for i := 0; i < 32; i++ {
		invalidUpdateKeyData[i] = byte(letters[rand.Intn(len(letters))]) //nolint:gosec // This is a test and doesn't need strong randomness
	}
	invalidUpdateSecret := base64.StdEncoding.EncodeToString(invalidUpdateKeyData)

	cfg.Listen = &addrPort
	cfg.Nameservers = []string{"dns4acme.example.com"}
	cfg.Backend = inmemory.ID
	cfg.BackendConfigs[inmemory.ID] = inmemory.Config{
		Keys: map[string]*backend.ProviderKeyResponse{
			"test": {
				Secret: updateSecret,
				Zones:  []string{"example.com"},
			},
			"notauth": {
				Secret: updateSecret,
				Zones:  []string{},
			},
		},
		Zones: map[string]*backend.ProviderZoneResponse{
			"example.com": {
				Serial:               0,
				ACMEChallengeAnswers: nil,
				Debug:                true,
			},
		},
	}

	ctx := t.Context()

	srv, err := dns4acme.New(
		ctx,
		cfg,
		testlogger.NewWriter(t),
	)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	t.Logf("Starting DNS4ACME...")
	started, err := srv.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		t.Logf("Stopping DNS4ACME...")
		if err := started.Stop(context.Background()); err != nil {
			t.Fatalf("Failed to stop server (%v)", err)
		}
	}()

	t.Run("query-soa", func(t *testing.T) {
		t.Logf("Querying SOA record...")
		msg := &dns.Msg{}
		msg.SetQuestion("_acme-challenge.example.com.", dns.TypeSOA)

		r, err := dns.Exchange(msg, addrPort.String())
		if err != nil {
			t.Fatalf("Failed to exchange: %v", err)
		}
		if len(r.Answer) != 1 {
			t.Fatalf("Expected 1 answer, got %d", len(r.Answer))
		}
		soa := r.Answer[0].(*dns.SOA)
		if soa.Serial != 0 {
			t.Fatalf("Expected serial 0, got %d", soa.Serial)
		}
		t.Logf("Correct SOA record received: %s", soa.String())
	})
	t.Run("query-txt", func(t *testing.T) {
		t.Logf("Querying TXT record...")
		msg := &dns.Msg{}
		msg.SetQuestion("_acme-challenge.example.com.", dns.TypeTXT)

		r, err := dns.Exchange(msg, addrPort.String())
		if err != nil {
			t.Fatalf("Failed to exchange: %v", err)
		}
		if len(r.Answer) != 0 {
			t.Fatalf("Expected 0 answer, got %d", len(r.Answer))
		}
		t.Logf("Received no TXT record, as expected.")
	})
	t.Run("update-txt-nosig", func(t *testing.T) {
		t.Logf("Trying to update TXT record without a signature...")
		msg := &dns.Msg{}
		msg.SetUpdate("_acme-challenge.example.com.")
		msg.Ns = []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "_acme-challenge.example.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Txt: []string{"test"},
			},
		}
		r, err := dns.Exchange(msg, addrPort.String())
		if err != nil {
			if errors.Is(err, dns.ErrAuth) {
				return
			}
			t.Fatalf("Failed to exchange: %v", err)
		}
		if r.Rcode != dns.RcodeNotAuth {
			t.Fatalf("Expected refusal, got %d", r.Rcode)
		}
		t.Logf("Received error, as expected.")
	})
	t.Run("update-txt-invalid-sig", func(t *testing.T) {
		t.Logf("Trying to update TXT record with an invalid signature...")
		msg := &dns.Msg{}
		msg.SetUpdate("_acme-challenge.example.com.")
		msg.Ns = []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "_acme-challenge.example.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Txt: []string{"test"},
			},
		}
		msg.SetTsig("test.", dns.HmacSHA256, 60, time.Now().Unix())
		cli := dns.Client{
			TsigSecret: map[string]string{
				"test.": invalidUpdateSecret,
			},
		}
		r, _, err := cli.Exchange(msg, addrPort.String())
		if err != nil {
			if errors.Is(err, dns.ErrAuth) {
				return
			}
			t.Fatalf("Failed to exchange: %v", err)
		}
		if r.Rcode != dns.RcodeNotAuth {
			t.Fatalf("Expected NOTAUTH, got %d", r.Rcode)
		}
		t.Logf("Received error, as expected.")
	})
	t.Run("update-txt-notauth", func(t *testing.T) {
		t.Logf("Trying to update TXT record with a correct signature, but without authorization...")
		msg := &dns.Msg{}
		msg.SetUpdate("_acme-challenge.example.com.")
		msg.Ns = []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "_acme-challenge.example.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Txt: []string{"notauth"},
			},
		}
		msg.SetTsig("notauth.", dns.HmacSHA256, 60, time.Now().Unix())
		cli := dns.Client{
			TsigSecret: map[string]string{
				"notauth.": updateSecret,
			},
		}
		r, _, err := cli.Exchange(msg, addrPort.String())
		if err != nil {
			if errors.Is(err, dns.ErrAuth) {
				return
			}
			t.Fatalf("Failed to exchange: %v", err)
		}
		if r.Rcode != dns.RcodeNotAuth {
			t.Fatalf("Expected NOTAUTH, got %d", r.Rcode)
		}
		t.Logf("Received error, as expected.")
	})
	t.Run("update-txt", func(t *testing.T) {
		t.Logf("Trying to update TXT record with a correct signature...")
		msg := &dns.Msg{}
		msg.SetUpdate("_acme-challenge.example.com.")
		msg.Ns = []dns.RR{
			&dns.TXT{
				Hdr: dns.RR_Header{
					Name:   "_acme-challenge.example.com.",
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Txt: []string{"test"},
			},
		}
		msg.SetTsig("test.", dns.HmacSHA256, 60, time.Now().Unix())
		cli := dns.Client{
			TsigSecret: map[string]string{
				"test.": updateSecret,
			},
		}
		r, _, err := cli.Exchange(msg, addrPort.String())
		if err != nil {
			if errors.Is(err, dns.ErrAuth) {
				return
			}
			t.Fatalf("Failed to exchange: %v", err)
		}
		if r.Rcode != dns.RcodeSuccess {
			t.Fatalf("Expected success, got %d", r.Rcode)
		}
		t.Logf("TXT record updated without error.")
	})
	t.Run("check-soa", func(t *testing.T) {
		t.Logf("Checking if the SOA record has been updated...")
		msg := &dns.Msg{}
		msg.SetQuestion("_acme-challenge.example.com.", dns.TypeSOA)

		r, err := dns.Exchange(msg, addrPort.String())
		if err != nil {
			t.Fatalf("Failed to exchange: %v", err)
		}
		if len(r.Answer) != 1 {
			t.Fatalf("Expected 1 answer, got %d", len(r.Answer))
		}
		soa := r.Answer[0].(*dns.SOA)
		if soa.Serial != 1 {
			t.Fatalf("Expected serial 1, got %d", soa.Serial)
		}
		t.Logf("Received SOA record: %s", soa.String())
	})
	t.Run("check-txt", func(t *testing.T) {
		t.Logf("Checking if the TXT record has been updated...")
		msg := &dns.Msg{}
		msg.SetQuestion("_acme-challenge.example.com.", dns.TypeTXT)

		r, err := dns.Exchange(msg, addrPort.String())
		if err != nil {
			t.Fatalf("Failed to exchange: %v", err)
		}
		if len(r.Answer) != 1 {
			t.Fatalf("Expected 1 answer, got %d", len(r.Answer))
		}
		txt := r.Answer[0].(*dns.TXT)
		if txt.Hdr.Name != "_acme-challenge.example.com." {
			t.Fatalf("Expected _acme-challenge.example.com. as a name, got %s", txt.Hdr.Name)
		}
		if len(txt.Txt) != 1 {
			t.Fatalf("Expected 1 TXT value, got %d", len(txt.Txt))
		}
		if txt.Txt[0] != "test" {
			t.Fatalf("Expected TXT record value of 'test', got %s", txt.Txt[0])
		}
		if txt.Hdr.Ttl != 60 {
			t.Fatalf("Expected ttl 60, got %d", txt.Hdr.Ttl)
		}
		t.Logf("Received TXT record: %s", txt.String())
	})
}
