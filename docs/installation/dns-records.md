# Setting up DNS records

Before you begin using DNS4ACME, you will need to set up a few DNS records with your main DNS provider. These records will designate the IP address for DNS4ACME and point your verification subdomain to it.

!!! note
    These steps need to be carried out with your main DNS provider, *not* in DNS4ACME.

## Pointing out the IP address for DNS4ACME

You can create an `A` record for this purpose. Assuming your domain is `example.com`, you would do the following:

```
dns4acme.example.com. IN A 1.2.3.4
```

If you have an IPv6 address, you can also create a `AAAA` record:

```
dns4acme.example.com. IN AAAA db80::1
```

## Setting up NS delegation

In the simplest scenario, you would want to create ACME certificates only for your main domain (e.g. `example.com`). To do this, you would now need to create an `NS` record for the `_acme-challenge.example.com` subdomain:

```
_acme-challenge.example.com. IN NS dns4acme.example.com.
```

!!! warning
    Do not change the `NS` records for your main domain (e.g.`example.com`), only for `_acme-challenge.example.com`. Changing the NS records for the main domain will make your domain unreachable! DNS4ACME is not a fully featured DNS server and is not suitable to manage all DNS records!

If you would like to host ACME verification for multiple domains, repeat this step for all domains.