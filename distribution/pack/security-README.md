# TLS material

CDS serves HTTPS only and refuses to start without a certificate and a private
key, so this directory is where they go:

```
server.crt      the CDS certificate, PEM
server.key      its private key, PEM
trust_store.pem the Identity Server certificate, written by `wso2cds enable`
```

`wso2cds enable` points `tls.cert_dir` in `repository/conf/deployment.yaml` at
this directory and exports the Identity Server's certificate into
`trust_store.pem`, so CDS can verify the server it calls.

The other direction needs `server.crt` to exist before enablement runs:
`wso2cds enable` imports it into the Identity Server's
`client-truststore.p12`, which is how that server verifies CDS when it calls
the synchronisation endpoints. Install the certificate and key first, or run
`wso2cds enable` again once they are in place.

The certificate is issued for the name CDS is reached on — the host in
`server_url` — not for `localhost`, unless that really is how the Identity
Server reaches it.
