---
title: cert-manager
---

# Integrating cert-manager with DNS4ACME

[Cert-manager](https://cert-manager.io/) is a Kubernetes certificate management tool [it supports sending RFC 2136 DNS updates](https://cert-manager.io/docs/configuration/acme/dns01/rfc2136/) to create DNS records.

Assuming you have cert-manager set up in your cluster, you can create the following record to point to DNS4ACME:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dns4acme
data:
  example.com: asdf # (1)!
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: example-issuer
spec:
  acme: # (2)!
    solvers:
      - dns01:
          rfc2136:
            nameserver: dns4acme.example.com # (3)!
            tsigKeyName: _acme-challenge.example.com # (4)!
            tsigAlgorithm: HMACSHA512 # (5)!
            tsigSecretSecretRef: # (6)!
              name: dns4acme
              key: example.com
```

1. Add your update key here.
2. See the cert-manager documentation for other fields needed here.
3. Point this to your DNS4ACME server.
4. Update this to match your domain exactly.
5. Use `HMACSHA256` or `HMACSHA512` here. Older signing algorithms such as `HMACMD5` are not supported.
6. Reference your secret from above here.