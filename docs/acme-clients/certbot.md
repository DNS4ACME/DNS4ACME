---
title: Certbot
---

# Integrating Certbot with DNS4ACME

!!! tip
    Alternative to this method, you can also integrate Certbot using the `--manual --manual-auth-hook` option. See the [`nsupdate` method](nsupdate.md) for details. 

If you would like Certbot to send DNS 2136 updates to DNS4ACME directly, you can use the [`certbot-dns-rfc2136` package](https://certbot-dns-rfc2136.readthedocs.io/en/stable/). Depending on your Certbot distribution, **you may need to install this package separately**.

## Creating a config file

First, you need to create a configuration file on how to update DNS4ACME:

```ini
dns_rfc2136_server = 192.0.2.1 ; (1)!
dns_rfc2136_port = 53 ; (2)!
dns_rfc2136_name = _acme-challenge.yourdomain.com ; (3)!
dns_rfc2136_secret = ... ; (4)!
dns_rfc2136_algorithm = HMAC-SHA512 ; (5)!
```

1. This should point to your DNS4ACME server.
2. Typically, this remains on port 53.
3. This name must match your domain name exactly.
4. This is your secret that you configured in the DNS4ACME backend.
5. Signing algorithm. Must be `HMAC-SHA256` or `HMAC-SHA512`. Older signing algorithms such as `HMAC-MD5` are not supported.

## Creating a certificate

Now you can create a certificate:

```
certbot certonly \
  --dns-rfc2136 \
  --dns-rfc2136-credentials path/to/your/config.ini \
  -d example.com
```

!!! note
    Certbot currently only supports using one key per run. This may not work with DNS4ACME because the name of the key
    may have to be different per domain.