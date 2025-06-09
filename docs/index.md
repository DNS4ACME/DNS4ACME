---
title: Home
hide:
- navigation
- toc
---

<p style="text-align:center;"><img src="logo.svg" alt="" /></p>

<h1 style="text-align:center">DNS4ACME - Bridging the cloud-native DNS gap</h1>

DNS4ACME is a lightweight DNS server that allows you to create DNS-verified certificates with ACME providers such a Let's Encrypt. DNS4ACME can help if you have an old-fashioned DNS server, and you cannot automatically create the DNS records needed for [DNS verification](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge).

!!! note
    Typically, you do not need DNS4ACME. HTTP verification is *much* easier to set up than DNS4ACME. DNS4ACME is only useful if you need to use DNS verification (e.g. for wildcard certificates) *and* your main DNS server does not have an API.

<div style="text-align:center;" markdown>

[Installing DNS4ACME &raquo;](installation/index.md){ .md-button } [Configuring DNS4ACME &raquo;](configuration/index.md){ .md-button } [Supported ACME clients &raquo;](acme-clients/index.md){ .md-button }

</div>

## How it works

To use DNS4ACME, you will need to set it up on a subdomain, such as `dns4acme.example.com`. This subdomain will point to the IP address you are running DNS4ACME on.

Now you can use subdomain delegation (`NS` record) for `_acme-challenge.example.com` to point that subdomain to your DNS4ACME instance. Your ACME client (e.g. [Certbot](https://certbot-dns-rfc2136.readthedocs.io/en/stable/) or the [cert-manager](https://cert-manager.io/)) sends RFC-2136 DNS updates to DNS4ACME to create DNS records.

![An image showing 4 boxes. In the top left corner the box is labeled ns1.yourprovider.com, with a second box inside labeled example.com. In the bottom left corner the box is labeled dns4acme.example.com, with a second box inside labeled _acme-challenge.example.com. There is an arrow between the top left inner box and the bottom left inner box labeled "IN NS". On the right side there are two boxes labeled "ACME service, e.g. Let's Encrypt" and "ACME client, e.g. Certbot". There is an arrow pointing from the bottom box to the top box. There is also an arrow from the bottom right box pointing to the bottom left outer box labeled "DNS updates".](dns4acme.svg)

You can achieve this delegation by creating the following DNS records in your "classic" server:

```
dns4acme.example.com. IN A 1.2.3.4 # <-- Insert the IP address here
_acme-challenge.example.com. IN NS dns4acme.example.com.
```

!!! note
    DNS4ACME is not a fully featured DNS server. It only supports creating the records necessary for ACME. If you need a full DNS server, please take a look at other alternatives, such as [PowerDNS](https://www.powerdns.com/). Many DNS providers these days also offer an API.