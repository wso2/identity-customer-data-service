# Local Development Setup

`scripts/local-setup/script.sh` sets up CDS with a WSO2 Identity Server for
local development.

A stock IS pack has no CDS configuration, so the script does the wiring: it
builds CDS and the IS extension bundles, generates and exchanges the TLS
certificates, writes both configurations, registers the OAuth applications CDS
needs, enables CDS for the organization, starts both servers and runs smoke
tests in both directions.

```bash
# Inbuilt SQLite database, no external dependencies
./scripts/local-setup/script.sh up

# CDS on PostgreSQL, started as a Docker container
./scripts/local-setup/script.sh up --db postgres
```

It prints the Console URL, the CDS URL and the client credentials it created.
The **Customer Data** section in the Console at `https://localhost:9443/console`
(`admin` / `admin`) reads live data from CDS.

```bash
./scripts/local-setup/script.sh status         # what is running
./scripts/local-setup/script.sh logs cds       # follow a log (cds | is)
./scripts/local-setup/script.sh down           # stop
```

`--help` lists all options.

---

## Starting an existing setup

`up` provisions. Once it has succeeded for a work directory, the pack,
applications, certificates and configuration are still there, and `start`
reuses them:

```bash
./scripts/local-setup/script.sh start          # ~20 seconds
./scripts/local-setup/script.sh restart        # stop, then start
```

`start` builds nothing, clones nothing, runs no Maven and calls no management
API. It starts the PostgreSQL container if the datasource needs one, starts both
servers, checks that CDS is still enabled for the organization and prints the
same summary.

It needs no options either: `up` records the datasource, ports, offset and
tenant it ran with, and `start`, `restart`, `down`, `status` and `logs` reuse
them. Options on the command line still win.

The smoke tests do not run, since they create and delete an IS user; `--tests`
runs them. To pick up a CDS code change, re-run `up` — it rebuilds the binary,
restarts CDS and leaves the Identity Server alone.

---

## Requirements

`curl`, `jq`, `unzip`, `openssl`, `python3` (with PyYAML), `git`, `go`, `java`
(11–21), `keytool`, `lsof` and `maven`. `docker` is needed for `--db postgres`.
Building the IS pack from source needs a few GB of free disk.

The Identity Server supports Java 11 to 21. The script picks an installed JDK in
that range over the machine default and warns if it cannot; an exported
`JAVA_HOME` wins.

---

## Work directory

Everything the script creates lives in one directory: `.local-dev/` next to the
repository by default, `--work-dir PATH` to move it. The repository is not
modified.

```text
<work-dir>/
  src/product-is/                 checkout the IS pack is built from
  src/identity-customer-data-service-extensions/
  is/wso2is-<version>/            the extracted pack, used as IS_HOME
  bin/cds                         the CDS binary
  cds-home/
    repository/conf/deployment.yaml     generated CDS configuration
    repository/database/cds.db          inbuilt database, with --db sqlite
    etc/certs/                          CDS key pair and the IS certificate
  logs/{cds,is,is-build}.log
  run/{cds,is}.pid
  state.env                       client IDs, secrets and the settings of the
                                  last successful `up`
```

`state.env` and the generated `deployment.yaml` hold client secrets and are
written `0600`. Both are outside the repository and are not meant to be
committed.

Use a separate `--work-dir` per setup to keep several side by side;
`down --purge` deletes one.

---

## The Identity Server pack

The Console renders the **Customer Data** section only if the IS build behind it
knows the `cds_host` configuration key, so the pack has to be recent. By default
the script builds one from
[`wso2/product-is`](https://github.com/wso2/product-is): a shallow clone plus
`mvn clean install -Dmaven.test.skip=true`, producing `wso2is-<version>.zip`.
Only `github.com` and the public WSO2 Maven repository are used.

The first build takes 10–30 minutes and a few GB of downloads, mostly Maven
populating `~/.m2`; a warm rebuild takes about a minute. Later runs reuse the
extracted pack and build nothing. `--is-rebuild` builds again, `--clean`
re-extracts.

| Option | |
|---|---|
| `--is-zip PATH` | use an existing pack and skip the build |
| `--is-src PATH` | build an existing `product-is` checkout |
| `--is-ref REF` | branch or tag to build (default `master`) |
| `--is-rebuild` | rebuild even when a pack was built earlier |

For a second work directory, `--is-zip` is the fastest option: a previous run
left its pack in `<work-dir>/src/product-is/modules/distribution/target/`.

---

## Ports

| | Default | Option |
|---|---|---|
| CDS (HTTPS) | 8900 | `--cds-port` |
| IS (HTTPS / HTTP) | 9443 / 9763 | `--is-offset N` |
| PostgreSQL | 5432 | `--pg-port` |

`up` fails if a port is taken; `--force` kills whatever holds it. `--is-offset N`
shifts both IS ports by `N`, so the server can run beside an existing one. A
second concurrent setup needs its own `--work-dir`, `--is-offset`, `--cds-port`
and `--pg-container`.

---

## What the setup configures

- **The extension bundles.** The four `org.wso2.identity.customer.data.service.*`
  jars in `repository/components/dropins`, with the `[[event_handler]]`
  subscriptions they need: `AbstractEventHandler.canHandle()` returns false
  without a module config, so a deployed bundle receives no events.
- **The CDS API resources and their scopes**, seeded through `deployment.toml`.
  IS reserves the `internal_` prefix and rejects it over REST, so the
  `internal_cds_*` scopes cannot be created through the management API.
- **`[console.extensions] cds_host`**, which makes the Console call CDS directly
  instead of building CDS URLs off the IS origin. Those cross-origin calls are
  allowed by the CDS `auth.cors_allowed_origins`.
- **Certificates in both directions.** The CDS certificate into the IS
  `client-truststore.p12`, the IS certificate into the CDS trust store. CDS does
  not start with an unreadable `tls.trust_store`, and neither self-signed
  certificate is in the system roots.
- **The `http://wso2.org/claims/cdsProfile` local claim** and its SCIM2 mapping.
  Without it, user events carry no profile cookie.
- **A relaxed policy on `/oauth2/introspect`**, so CDS introspects the tokens it
  receives with its own client credentials instead of admin-user Basic auth.
- **Two OAuth applications**: a system app for the calls CDS makes into IS, and
  a client app with `aud=iam-cds`, the audience CDS requires, for calling the
  CDS APIs. The Console is handled separately because it issues opaque tokens.
- **`cds_enabled` for the organization.** Every profile, schema and rule
  endpoint returns `400` until it is set. Setting it also runs the initial
  profile-schema sync and seeds the default consent category.

---

## Directory layout

`script.sh` handles arguments, ordering and process control. The configuration
it generates is kept in `templates/` next to it, one file per artifact:

| Template | Written to |
|---|---|
| `is-deployment.toml` | appended to the pack's `repository/conf/deployment.toml`, between `# BEGIN/END cds-local-dev` markers so that a re-run replaces it |
| `cds-deployment.yaml` | merged onto `config/repository/conf/deployment.yaml` to produce `<work-dir>/cds-home/repository/conf/deployment.yaml` |
| `openssl.cnf` | the request the CDS certificate is generated from |

`lib/` holds the scripts that render and patch them: `render.py` fills in
placeholders, `render_cds_config.py` merges the overlay onto the shipped CDS
config, and `patch_is_toml.py` replaces the generated `deployment.toml` block
and sets the `[server]` offset. Each runs on its own, which is the quickest way
to check a template change without a full `up`.

Change what the setup configures by editing a template, not the script. Values
are `${NAME}` placeholders and a placeholder with no value is an error;
`${int:NAME}` in `cds-deployment.yaml` produces a number rather than a string.

---

## Smoke tests

`up` ends by checking the integration in both directions. `--skip-tests` skips
them.

1. `GET /cds/api/v1/ready` returns 200 — CDS is up and its database is reachable
2. a client-credentials token carries `aud=iam-cds`
3. the same token carries the expected `org_handle`
4. CDS reports the organization as enabled
5. `GET /profiles` returns 200 — introspection and scope mapping work
6. `GET /profile-schema` returns 200 — CDS reached the IS claim APIs
7. a user created in IS appears as a profile in CDS — dropins, event handlers
   and the IS to CDS sync path work

Test 7 creates a user and deletes it again; `--keep-test-user` keeps it.

---

## Troubleshooting

**No Customer Data section in the Console.** The pack predates CDS support in
the Console. Its configuration template and its web app ship together, so an
older pack cannot be configured into working; build a current one with
`--is-rebuild`. `up` reports `bundled Console supports [console.extensions]
cds_host` when the pack is recent enough.

**Every CDS endpoint returns 400.** CDS is not enabled for the organization.
`logs/cds.log` shows `CDS is not enabled for organization`; re-running `up`
fixes it.

**`GET /profile-schema` returns 400.** CDS could not reach the IS claim APIs,
usually the trust store or the system application. `logs/cds.log` has the error.

**A user in IS never appears in CDS.** Check that the four bundles are in
`repository/components/dropins` and that `logs/is.log` shows the CDS handlers
registering; an unresolved dropin is otherwise silent.

**The build fails.** `logs/is-build.log` has the Maven output. A JDK outside
11–21 is the usual cause.

**`down` reports nothing running but the ports are held.** Something outside the
script owns them. `status` reports what the script knows about; `--force` on the
next `up` clears the rest.

---

## Scope

This is a development setup, not a deployment reference.

- Only the CDS datasource is configurable. The Identity Server keeps its default
  H2 databases.
- CDS runs with the in-memory queue. See
  [Extending Queue Providers](extending-queue-providers.md) for ActiveMQ and
  other brokers.
- The certificates are self-signed and the credentials are the defaults, so
  clients talk to both servers with verification off.

The [README](../../README.md) documents the same setup done by hand, and covers
anything the script does not.
