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
)

type tsigProvider struct {
	backend backend.Provider
	ctx     context.Context
}

func (r tsigProvider) Generate(msg []byte, t *dns.TSIG) ([]byte, error) {
	zoneData, err := getZone(r.ctx, r.backend, t.Hdr.Name)
	if err != nil {
		return nil, ErrTSigFailure.Wrap(err)
	}
	return r.generateSignature(zoneData.UpdateKey, msg, t)
}

func (r tsigProvider) Verify(msg []byte, t *dns.TSIG) error {
	zoneData, err := getZone(r.ctx, r.backend, t.Hdr.Name)
	if err != nil {
		return ErrTSigFailure.Wrap(err)
	}
	return r.hmacVerify(msg, zoneData.UpdateKey, t)
}

func (r tsigProvider) hmacVerify(msg []byte, key string, t *dns.TSIG) error {
	b, err := r.generateSignature(key, msg, t)
	if err != nil {
		return ErrTSigFailure.Wrap(err)
	}
	mac, err := hex.DecodeString(t.MAC)
	if err != nil {
		return ErrTSigFailure.Wrap(err)
	}
	if !hmac.Equal(b, mac) {
		return ErrTSigFailure.Wrap(dns.ErrSig)
	}
	return nil
}

func (r tsigProvider) generateSignature(key string, msg []byte, t *dns.TSIG) ([]byte, error) {
	decodedKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, ErrTSigFailure.Wrap(err)
	}
	var h hash.Hash
	switch dns.CanonicalName(t.Algorithm) {
	case dns.HmacSHA256:
		h = hmac.New(sha256.New, decodedKey)
	case dns.HmacSHA512:
		h = hmac.New(sha512.New, decodedKey)
	default:
		return nil, ErrTSigFailure.Wrap(dns.ErrKeyAlg)
	}
	h.Write(msg)
	return h.Sum(nil), nil
}
