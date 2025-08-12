package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"github.com/dns4acme/dns4acme/backend"
	"github.com/dns4acme/dns4acme/lang/E"
	"github.com/miekg/dns"
	"hash"
	"log/slog"
	"strings"
)

// TODO the error messages here are not particularly useful due to the lacking context.

type tsigProvider struct {
	logger  *slog.Logger
	backend backend.Provider
	ctx     context.Context
}

func (r tsigProvider) Generate(msg []byte, t *dns.TSIG) ([]byte, error) {
	keyName := strings.TrimSuffix(t.Hdr.Name, ".")
	key, err := r.backend.GetKey(r.ctx, keyName)
	if err != nil {
		r.logger.DebugContext(r.ctx, "Error getting key", E.ToSLogAttr(err, slog.String("key", keyName))...)
		return nil, dns.ErrSig
	}
	return r.generateSignature(keyName, key.Secret, msg, t)
}

func (r tsigProvider) Verify(msg []byte, t *dns.TSIG) error {
	keyName := strings.TrimSuffix(t.Hdr.Name, ".")
	key, err := r.backend.GetKey(r.ctx, keyName)
	if err != nil {
		return err
	}
	return r.hmacVerify(keyName, msg, key.Secret, t)
}

func (r tsigProvider) hmacVerify(keyName string, msg []byte, key string, t *dns.TSIG) error {
	b, err := r.generateSignature(keyName, key, msg, t)
	if err != nil {
		return err
	}
	mac, err := hex.DecodeString(t.MAC)
	if err != nil {
		r.logger.DebugContext(r.ctx, "Cannot hex-decode tsig MAC", E.ToSLogAttr(err)...)
		return err
	}
	if !hmac.Equal(b, mac) {
		return dns.ErrSig
	}
	return nil
}

func (r tsigProvider) generateSignature(keyName string, key string, msg []byte, t *dns.TSIG) ([]byte, error) {
	decodedKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, ErrInvalidTsigKey.Wrap(err).WithAttr(slog.String("key", keyName))
	}
	var h hash.Hash
	switch dns.CanonicalName(t.Algorithm) {
	case dns.HmacSHA256:
		h = hmac.New(sha256.New, decodedKey)
	case dns.HmacSHA512:
		h = hmac.New(sha512.New, decodedKey)
	default:
		r.logger.DebugContext(r.ctx, "Cannot generate signature, unsupported algorithm", slog.String("algorithm", t.Algorithm))
		return nil, ErrUnsupportedTsigAlgorithm.Wrap(dns.ErrSig).WithAttr(slog.String("key", keyName))
	}
	h.Write(msg)
	return h.Sum(nil), nil
}
