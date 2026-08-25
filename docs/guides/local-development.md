# Local Development Setup — CDS + Identity Server in One Command

`scripts/local-setup/script.sh` brings up CDS **and** a fully wired WSO2 Identity
Server with no manual steps.

A stock IS pack knows nothing about CDS, so the script does all of the wiring
itself: it builds CDS and the IS extension bundles, generates and exchanges the
TLS certificates, writes both servers' configuration, registers the OAuth
applications CDS needs, enables CDS for the organization, starts both servers
and smoke-tests the integration in both directions.

```bash
# Inbuilt SQLite database — no external dependencies at all
./scripts/local-setup/script.sh up

# Or run CDS on PostgreSQL (started as a Docker container)
./scripts/local-setup/script.sh up --db postgres
```

When it finishes it prints the Console URL, the CDS URL and the client
credentials it created. Open the Console at `https://localhost:9443/console`
(`admin` / `admin`) and the **Customer Data** section is there, reading live
data from CDS.

```bash
./scripts/local-setup/script.sh status         # what is running
./scripts/local-setup/script.sh logs cds       # follow a log (cds | is)
./scripts/local-setup/script.sh down           # stop everything
```

`--help` lists every option; the ones worth knowing about are below.

---

## Day to day: `start`

`up` is the setup command. Once it has succeeded for a work directory,
everything it produced is still there, so bringing the same environment back up
needs none of it:

```bash
./scripts/local-setup/script.sh start          # ~20 seconds, no setup steps
./scripts/local-setup/script.sh restart        # stop, then the same quick start
```

`start` builds nothing, clones nothing, runs no Maven and calls no management
API. It starts the PostgreSQL container if the datasource needs one, starts both
servers, confirms CDS is still enabled for the organization and prints the same
summary — reusing the OAuth applications, certificates and configuration the
`up` left behind.

It also needs no options. `up` records what it ran with in the work directory,
so `start`, `restart`, `down`, `status` and `logs` all pick the same datasource,
ports, offset and tenant back up. Anything passed on the command line still
wins.

Two things `start` deliberately does not do:

- **No smoke tests.** They create and delete an IS user, and the point of the
  command is to be quick. `--tests` runs them anyway.
- **No rebuild.** To pick up a CDS code change, re-run `up`: it rebuilds the
  binary, restarts CDS and leaves the Identity Server alone. Re-running `up` is
  always safe — every step finds-or-creates rather than duplicating.

---

## Requirements

`curl`, `jq`, `unzip`, `openssl`, `python3` (with PyYAML), `git`, `go`, `java`
(11–21), `keytool` and `maven`. `docker` is only needed for `--db postgres`.
Building the IS pack from source also wants a few GB of free disk.

The Identity Server supports Java 11 to 21. The script prefers an installed JDK
in that range over whatever the machine defaults to, and warns if it has to use
something else — an already-exported `JAVA_HOME` always wins.

---

## The work directory

Everything the script creates lives in one throwaway directory — `.local-dev/`
next to the repository by default, `--work-dir PATH` to move it. The repository
itself is never modified.

```
<work-dir>/
  src/product-is/                 checkout the IS pack is built from
  src/identity-customer-data-service-extensions/
  is/wso2is-<version>/            the extracted pack — this is IS_HOME
  bin/cds                         the CDS binary
  cds-home/
    repository/conf/deployment.yaml     generated CDS configuration
    repository/database/cds.db           inbuilt database, if --db sqlite
    etc/certs/                           CDS key pair and the IS certificate
  logs/{cds,is,is-build}.log
  run/{cds,is}.pid
  state.env                       client IDs, secrets, and the settings
                                  the last successful `up` ran with
```

`state.env` is also what makes `start` need no options — see above. It and the
generated `deployment.yaml` hold real client secrets, so
both are written `0600`. They live outside the repository and nothing there is
meant to be committed or shared.

Use a separate `--work-dir` per configuration you want to keep side by side;
`down --purge` deletes one.

---

## Where the Identity Server pack comes from

The Console only renders the **Customer Data** section if the IS build behind it
knows the `cds_host` configuration key, so the script needs a recent pack. By
default it builds one from [`wso2/product-is`](https://github.com/wso2/product-is):
a shallow clone plus `mvn clean install -Dmaven.test.skip=true`, which produces
`wso2is-<version>.zip`. Only `github.com` and the public WSO2 Maven repository
are involved, so it works from any network.

Budget 10–30 minutes and a couple of GB of downloads for the first build — it is
almost all Maven populating `~/.m2`. Measured on an M-series Mac: 8m39s with a
partly warm cache, 69s to rebuild once warm.

Later runs skip all of it: an already-extracted pack short-circuits the step, so
nothing is cloned or built. To force a new pack, use `--is-rebuild` (rebuild) or
`--clean` (re-extract).

| Option | |
|---|---|
| `--is-zip PATH` | use a pack that is already on disk and skip the build entirely |
| `--is-src PATH` | build a `product-is` checkout you already have |
| `--is-ref REF` | branch or tag to build (default `master`) |
| `--is-rebuild` | rebuild even when a pack was built earlier |

`--is-zip` is the fastest way to set up a second work directory — the pack a
previous run built is sitting in
`<work-dir>/src/product-is/modules/distribution/target/`.

---

## Ports

| | Default | Change it with |
|---|---|---|
| CDS (HTTPS) | 8900 | `--cds-port` |
| IS (HTTPS / HTTP) | 9443 / 9763 | `--is-offset N` |
| PostgreSQL | 5432 | `--pg-port` |

`up` fails if a port is already taken. `--force` kills whatever holds it;
`--is-offset N` shifts both IS ports by `N` so the new server runs beside an
existing one. To run a second full setup concurrently, give it its own
`--work-dir`, `--is-offset`, `--cds-port` and `--pg-container`.

---

## What it wires up

Worth knowing, because these are the pieces that are easy to miss when setting
this up by hand:

- **The extension bundles.** The four `org.wso2.identity.customer.data.service.*`
  jars go into `repository/components/dropins`, together with the
  `[[event_handler]]` subscriptions without which the handlers never receive an
  event — `AbstractEventHandler.canHandle()` returns false with no module config,
  so the bundles load and then sit silent.
- **The CDS API resources and their scopes**, seeded through `deployment.toml`.
  IS reserves the `internal_` prefix and rejects it over REST, so the
  `internal_cds_*` scopes cannot be created through the management API.
- **`[console.extensions] cds_host`**, which is what makes the Console call CDS
  directly instead of building CDS URLs off the IS origin. CDS answers those
  cross-origin calls through its `auth.cors_allowed_origins`.
- **Certificates in both directions.** The CDS certificate goes into the IS
  `client-truststore.p12`, and the IS certificate into the CDS trust store —
  CDS refuses to start with an unreadable `tls.trust_store`, and neither
  self-signed certificate is in the system roots.
- **The `http://wso2.org/claims/cdsProfile` local claim** and its SCIM2 mapping.
  Without it, user events carry no profile cookie.
- **A relaxed policy on `/oauth2/introspect`**, so CDS can introspect the tokens
  it receives using its own client credentials rather than admin-user Basic auth.
- **Two OAuth applications** — a system app for the calls CDS makes into IS, and
  a client app with `aud=iam-cds` for calling CDS yourself. CDS hardcodes that
  audience, and the Console is handled separately because it issues opaque
  tokens.
- **`cds_enabled` for the organization.** Every profile, schema and rule endpoint
  returns `400` until this is set. Setting it also runs the initial
  profile-schema sync and seeds the default consent category.

---

## How the setup is organised

The configuration is not buried in the script. `script.sh` handles arguments,
ordering and process control; everything it writes lives beside it as a file:

```
scripts/local-setup/
  script.sh
  templates/
    is-deployment.toml       the CDS section appended to the pack's deployment.toml
    cds-deployment.yaml      merged onto config/repository/conf/deployment.yaml
    console-features.json    Console fallback, for packs predating `cds_host`
    openssl.cnf              the CDS certificate request
  lib/                       the helpers that render and patch the above
```

So changing what the setup configures means editing a template, not the script.
Values arrive as `${NAME}` placeholders and rendering fails if one has no value,
which keeps a half-filled configuration file from reaching a server.
[`scripts/local-setup/README.md`](../../scripts/local-setup/README.md) covers the
directory in detail.

---

## The smoke tests

`up` finishes by proving the integration works, in both directions. `--skip-tests`
skips them.

1. `GET /cds/api/v1/ready` returns 200 — CDS is up and its database is reachable
2. a client-credentials token carries `aud=iam-cds`
3. the same token carries the expected `org_handle`
4. CDS reports the organization as enabled
5. `GET /profiles` returns 200 — introspection and scope mapping work
6. `GET /profile-schema` returns 200 — CDS successfully called the IS claim APIs
7. a user created in IS appears as a profile in CDS — dropins, event handlers and
   the IS→CDS sync path all work

Test 7 creates a user and deletes it again; `--keep-test-user` leaves it in place.

---

## Troubleshooting

**No "Customer Data" section in the Console.** The pack predates `cds_host`. The
script says which case it hit — `bundled Console supports [console.extensions]
cds_host` means the pack is fine. Build a current pack (`--is-rebuild`) rather
than patching the Console.

**Every CDS endpoint returns 400.** CDS is not enabled for the organization. Look
for `CDS is not enabled for organization` in `logs/cds.log`; re-running `up`
fixes it.

**`GET /profile-schema` returns 400.** CDS could not reach the IS claim APIs —
usually the trust store or the system application. `logs/cds.log` has the
underlying error.

**A user in IS never appears in CDS.** Check that the four bundles are in
`repository/components/dropins` and that `logs/is.log` shows the CDS handlers
registering; a dropin that fails to resolve is silent otherwise.

**The build fails.** `logs/is-build.log` has the full Maven output. A JDK outside
11–21 is the usual cause.

**`down` says nothing is running but ports are still held.** Something outside
the script owns them. `./scripts/local-setup/script.sh status` reports what the
script knows about; `--force` on the next `up` clears the rest.

---

## Scope

This is a development setup, not a deployment reference.

- Only CDS's datasource is configurable. The Identity Server keeps its default
  H2 databases.
- CDS runs with the in-memory queue. See
  [Extending Queue Providers](extending-queue-providers.md) for ActiveMQ and
  other brokers.
- The certificates are self-signed and the credentials are the defaults, so
  clients talk to both servers with verification off.

The [README](../../README.md) documents the same setup done by hand, which is
the reference for anything the script does not cover.
