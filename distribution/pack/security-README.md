# TLS material

CDS serves HTTPS only and refuses to start without a certificate and a private
key, so this directory holds them:

```
server.crt      the CDS certificate, PEM
server.key      its private key, PEM, mode 600
trust_store.pem the Identity Server certificate, written by `wso2cds enable`
```

## Replace the certificate this pack shipped with

`server.crt` and `server.key` are a **default key pair, generated when this
pack was built**, so that a fresh installation starts and enables in one pass.
They are a convenience for getting started and nothing else.

**Every download of this release carries the same private key.** Anyone holding
it can present themselves as this service to the Identity Server, which trusts
the certificate after `wso2cds enable` imports it. Replace both files with a
certificate issued for the name CDS is reached on, then run `wso2cds enable`
again so the Identity Server trusts the new one.

`wso2cds status` and `wso2cds verify` both report while the default is still in
use, and `verify` counts it as a failed check — it is not something to leave in
place on a deployment carrying real traffic.

## What enable does here

`wso2cds enable` points `tls.cert_dir` in `repository/conf/deployment.yaml` at
this directory, exports the Identity Server's certificate into
`trust_store.pem` so CDS can verify the server it calls, and imports
`server.crt` into the Identity Server's `client-truststore.p12` so that server
can verify CDS when it calls the synchronisation endpoints.

Replacing the certificate therefore means running `wso2cds enable` again: the
Identity Server trusts whichever certificate was imported, not whichever one
is on disk now.

A replacement should be issued for the host in `server_url` — the name the
Identity Server and the Console reach CDS on — not for `localhost`, unless
that really is how they reach it.
