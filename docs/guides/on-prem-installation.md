# Installing CDS on-premises

CDS runs as its own process beside a WSO2 Identity Server. The two are
separate installations with separate lifecycles: CDS never starts, stops or
updates the Identity Server, and the Identity Server knows about CDS only
through the configuration and extension bundles that enablement puts there.

Everything below is done with `bin/wso2cds`, the single executable a pack
ships.

---

## The two steps that are not the same step

**Installation** establishes `CDS_HOME`. For a direct download, extracting the
zip *is* the installation — there is no `setup.sh` and no `wso2cds install`.

**Enablement** establishes the relationship between that installation and one
Identity Server. It is a separate command because it is a separate decision,
and because it is the only part that writes into `IS_HOME`.

---

## Installing

A release publishes one pack per supported platform:

```bash
unzip wso2cds-linux-amd64-1.2.0.zip -d /opt
cd /opt/wso2cds-linux-amd64-1.2.0

# Review the licence and required notices.
less LICENSE.txt

# Install the TLS certificate and key — see repository/resources/security/README.md.
cp server.crt server.key repository/resources/security/

./bin/wso2cds enable --is-home /opt/wso2is
```

## What enablement does

It runs in two phases, because the Identity Server has to restart between
them.

**Before the restart**, writing into `IS_HOME`:

| | |
|---|---|
| Extension bundles | installs any the Identity Server does not already provide into `repository/components/dropins/` |
| Connection | writes the CDS endpoint and the synchronisation credentials |
| Event handlers | subscribes the user, claim, authentication and session events CDS needs |
| CDS system APIs | seeds the API resources and their `internal_cds_*` scopes |
| Console | configures the Customer Data section where the bundled Console supports it |
| Certificates | exports the Identity Server certificate for CDS, and imports the CDS certificate into `client-truststore.p12` |

The generated configuration goes into `repository/conf/deployment.toml`
between `# BEGIN wso2cds` and `# END wso2cds` markers. Everything outside them
is preserved exactly, a backup is kept at `deployment.toml.cds-backup`, and
re-enabling replaces the block rather than appending a second copy.

### The extension bundles are the Identity Server's, not ours

From the Identity Server release that ships CDS support, the four extension
bundles arrive with the product and are delivered and updated through **its**
update stream. Enablement therefore looks in `dropins/` first and leaves alone
anything it finds:

```
CDS extension bundles
    org.wso2.identity.customer.data.service.client-0.9.0.jar (provided by the Identity Server)
    ...
    + the Identity Server provides all 4 CDS extension bundles
```

Replacing one would put a file CDS downloaded where the Identity Server
expects its own, and removing it on `disable` would leave that installation
short of a bundle it shipped with. So `disable` removes only the bundles CDS
installed, and `status` reports which came from where — the answer decides who
updates them.

On an Identity Server from before CDS support, none are there and CDS installs
the whole baseline. A mixture is handled the same way, bundle by bundle.

If the Identity Server provides a bundle at a different version from the one
this CDS release expects, enablement says so and tells you which Identity
Server update carries the right one. It does not install over it: that
installation's updates are not CDS's to apply.

`--extensions <dir>` is the exception, and the one case where an
Identity-Server-provided bundle is replaced — it is you naming the bundles to
use, for an unreleased build or a damaged installation, and it says what it
replaced.

The command then reports that a restart is required, and stops. It does not
restart the Identity Server — that installation is managed separately, and
restarting it is your decision:

```bash
/opt/wso2is/bin/wso2server.sh restart
```

**After the restart**, over the Identity Server's management APIs:

1. verify the CDS API resources and scopes were registered
2. create or reuse the one application the integration uses, authorise it for
   the Identity Server APIs CDS calls and for the CDS APIs, and give it the
   `iam-cds` audience
3. authorise the Console application and the administrator role
4. enable CDS for the organization

No application is created for your own API consumers. Which clients may call
CDS, and with which scopes, is yours to decide — enablement establishes the
relationship between CDS and one Identity Server and goes no further.

The profile claim the CDS extension bundles use is not created here. It
belongs to those bundles, which is where it is declared and consumed; CDS
itself never reads it.

Run the same command again to reach this phase:

```bash
./bin/wso2cds enable
```

It needs an Identity Server administrator password, taken from
`--admin-password`, `$CDS_IS_ADMIN_PASSWORD`, or a prompt.

Enablement is idempotent and resumable throughout: every step checks what is
already there before it changes anything, so re-running after a failure
continues rather than duplicating.

### Starting, and the last step

Step 6 is a call to the CDS configuration API, so it needs CDS to be running.
Starting CDS is an explicit lifecycle operation and enablement will not do it
for you, so the sequence at the end is:

```bash
./bin/wso2cds start
./bin/wso2cds enable     # completes organization enablement
./bin/wso2cds verify
```

Enabling the organization is also what runs the first profile-schema
synchronisation and seeds the default consent categories. Until it has
happened, the profile, schema and rule endpoints refuse requests.

---

## Running CDS

| Command | What it does |
|---|---|
| `wso2cds start` | starts the runtime, detached, and waits for it to report ready |
| `wso2cds start --debug` | runs it in the foreground, logging to the terminal |
| `wso2cds stop` | stops it, allowing a graceful shutdown before killing it |
| `wso2cds restart` | stop, then start |
| `wso2cds status` | product, platform, runtime and integration state |
| `wso2cds logs -f` | follows `logs/wso2cds.log` |
| `wso2cds verify` | checks the deployment without changing it |

Starting an installation that was never enabled is refused, with the command
that would fix it. Nothing is downloaded or configured on your behalf.

### Status

```
$ ./bin/wso2cds status

  CDS Version         1.2.0
  CDS Home            /opt/wso2cds
  CDS U2 Product      wso2cds-linux-amd64-1.2.0
  CDS U2 Level        0004
  Platform            linux/amd64
  CDS Process         Running (pid 24113)
  CDS Endpoint        https://127.0.0.1:8900
  CDS Readiness       Ready
  Associated IS       7.4.0 (/opt/wso2is)
  IS U2 State         Managed independently from IS_HOME
  Organization        carbon.super
  Extension Bundles   4, all provided by the Identity Server
  Extension Staged    0.9.0.1
  Extension Active    0.9.0
  IS Restart Pending  Yes
```

Staged and active are reported separately on purpose. An Identity Server
update can put a new CDS extension bundle in place while the running server
still has the old one loaded; a staged bundle is not active until that server
restarts and `verify` confirms it. Reporting one number for both would let an
installation look finished while a restart is still owed.

### Verification

`wso2cds verify` is read-only. It checks readiness and the datastore, that the
extension bundles are installed and active, that a CDS client token carries
the `iam-cds` audience, that an authorized `GET /profiles` succeeds for the
organization, and that `GET /profile-schema` succeeds — which is the one check
that exercises the CDS-to-Identity-Server direction.

---

## Disabling

```bash
./bin/wso2cds disable
```

This removes what enablement wrote into `IS_HOME`: the extension bundles **it
installed**, and the generated section of `deployment.toml`. Bundles that came
with the Identity Server are left in place. The CDS
installation, its configuration and its data are untouched, and the
applications, claims and role permissions provisioning created are left alone
because they may hold state you still want. The Identity Server stops calling
CDS once it restarts.

---

## Secrets

Credentials never go into `deployment.yaml` or the integration record. They go
to `repository/conf/secrets.env`, owner-readable only, and reach the runtime
as the environment variables the `${...}` placeholders in the configuration
expand from.

A deployment with an approved secret manager supplies the same variables from
there and can delete the file — anything already set in the environment wins
over it.

---

## Layout

```
<CDS_HOME>/
├── bin/
│   └── wso2cds                     the lifecycle command and the runtime
├── repository/
│   ├── conf/
│   │   ├── deployment.yaml         the runtime configuration
│   │   ├── compatibility.yaml      what this release works with
│   │   ├── integration.yaml        written by enable
│   │   └── secrets.env             written by enable, mode 600
│   ├── database/                   the inbuilt database, when one is used
│   ├── dbscripts/                  DDL for an external database
│   └── resources/security/         the TLS material
├── extensions/                     the bundles enable installs into the IS
├── logs/
├── updates/
├── version.txt
├── LICENSE.txt
└── README.md
```

`bin/` and `updates/` stay siblings at the pack root, which is what the update
tooling a release adds to the pack expects.

---

## Air-gapped installations

Enablement downloads the extension bundles from the WSO2 Maven repository the
first time it needs them, and caches them in `extensions/`. Nothing is fetched
for a bundle the Identity Server already provides, so an installation on a
release that ships them needs no access at all.

Where bundles are missing and there is no access to that repository, supply
them instead:

```bash
./bin/wso2cds enable --is-home /opt/wso2is --extensions /media/bundles
```

or by placing the jars in `<CDS_HOME>/extensions/` before enabling. Local
bundles are always preferred over the network.
