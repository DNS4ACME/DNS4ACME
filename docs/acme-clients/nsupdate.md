title: nsupdate

# Integrating DNS4ACME with nsupdate (scripting)

When your ACME client doesn't support RFC2136 DNS updates, you can work around this limitation by updating DNS4ACME using a script. To do this, you will need to install the `nsupdate` command line tool. You can then send updates to DNS4ACME like this:


```shell
UPD="server dns4acme.example.com 53"$'\n' # (1)!
UPD+="key hmac-sha256:_acme-challenge.example.com Eeheighup..."$'\n' # (2)!
UPD+="update delete _acme-challenge.example.com. 60 IN TXT"$'\n'
UPD+="update add _acme-challenge.example.com. 60 IN TXT \"value here\""$'\n' # (3)!
UPD+="send"$'\n'
echo "${UPD}" | nsupdate
```

1.  Point this to your DNS4ACME installation.
2.  You can use `hmac-sha256` or `hmac-sha512` here. Older signing methods, such as `hmac-md5` are not supported. The name of the key must match the FQDN for the ACME verification record.
3.  Enter the verification value here.