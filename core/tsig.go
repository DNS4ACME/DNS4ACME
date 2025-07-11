package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"github.com/dns4acme/dns4acme/backend"
	"github.com/miekg/dns"
	"hash"
	"strings"
)

type tsigProvider struct {
	backend backend.Provider
	ctx     context.Context
}

func (r tsigProvider) Generate(msg []byte, t *dns.TSIG) ([]byte, error) {
	key, err := r.backend.GetKey(r.ctx, strings.TrimSuffix(t.Hdr.Name, "."))
	if err != nil {
		return nil, dns.ErrSig
	}
	return r.generateSignature(key.Secret, msg, t)
}

func (r tsigProvider) Verify(msg []byte, t *dns.TSIG) error {
	key, err := r.backend.GetKey(r.ctx, strings.TrimSuffix(t.Hdr.Name, "."))
	if err != nil {
		return dns.ErrSig
	}
	return r.hmacVerify(msg, key.Secret, t)
}

func (r tsigProvider) hmacVerify(msg []byte, key string, t *dns.TSIG) error {
	b, err := r.generateSignature(key, msg, t)
	if err != nil {
		// TODO error handling
		return err
	}
	mac, err := hex.DecodeString(t.MAC)
	if err != nil {
		// TODO error handling
		return err
	}
	if !hmac.Equal(b, mac) {
		// TODO error handling
		return dns.ErrSig
	}
	return nil
}

func (r tsigProvider) generateSignature(key string, msg []byte, t *dns.TSIG) ([]byte, error) {
	decodedKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		// TODO log error
		return nil, dns.ErrSig
	}
	var h hash.Hash
	switch dns.CanonicalName(t.Algorithm) {
	case dns.HmacSHA256:
		h = hmac.New(sha256.New, decodedKey)
	case dns.HmacSHA512:
		h = hmac.New(sha512.New, decodedKey)
	default:
		// TODO log error
		return nil, dns.ErrSig
	}
	h.Write(msg)
	return h.Sum(nil), nil
}
