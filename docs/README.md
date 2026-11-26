# CDD Documentation

CDD, the Customer Data Directory, builds unified customer profiles from the
identity and behavioural data an organization already holds, and serves them to
the applications that personalize on them. It runs as its own service beside the
Identity Server.

## Getting started

1. **Download** the zip for your platform from the
   [releases page](https://github.com/wso2/identity-customer-data-service/releases):
   Linux, macOS or Windows.
2. **Extract it, fill in its secrets, and start it.** Extracting is the
   installation. The pack ships `repository/conf/secrets.env` with every value
   empty; set at least these two before the first start:

   ```bash
   unzip cdd-linux-amd64-<version>.zip
   cd cdd-linux-amd64-<version>
   openssl rand -base64 32   # the value for SERVICE_CREDENTIALS_ENCRYPTION_KEY
   vi repository/conf/secrets.env
   bin/start.sh        # bin\start.bat on Windows
   ```

   ```bash
   SERVICE_CREDENTIALS_ENCRYPTION_KEY=<generated for this installation>
   AUTH_SERVER_ADMIN_PASSWORD=<the admin_password of step 3>
   ```

   Generate the key for each installation and back it up; never reuse the one
   from another installation or from an example.

   CDD runs in the foreground on `https://localhost:8900`; Ctrl+C stops it. It
   starts on an inbuilt database, so nothing else has to be installed.
3. **Switch it on in the Identity Server**, by adding this to the Identity
   Server's `deployment.toml` and restarting it:

   ```toml
   [customer_data_service]
   enable = true
   server_url = "https://localhost:8900"
   admin_username = "admin"
   admin_password = "<AUTH_SERVER_ADMIN_PASSWORD in secrets.env>"
   ```

4. **Enable CDD for an organization** from the Identity Server Console.

The pack works out of the box with a default Identity Server on the same host.
Before production, replace the default certificate and move to PostgreSQL,
MySQL or SQL Server — the [installation guide](guides/on-prem-installation.md)
covers configuration, certificates, secrets and running CDD as a service.

## Architecture

| Document | Description |
|---|---|
| [Architecture](architecture.md) | How CDS is put together — request path, layering, data layer, workers, security, and the conventions that hold them together |

## Concepts

| Document | Description |
|---|---|
| [Profiles](concepts/profiles.md) | What a profile is, temporary vs permanent, lifecycle, cookies, application data |
| [Profile Schema](concepts/profile-schema.md) | Attribute types, mutability, merge strategies, core schema |
| [Consent](concepts/consent.md) | Consent categories, attribute scopes, consent-filtered profile fetch |
| [Unification Rules](concepts/unification-rules.md) | Rule structure, strict and tolerant matching, evidence strength, priority, rejections |
| [How Unification Works](concepts/how-unification-works.md) | The full merge pipeline — blocking index, per-attribute comparison, scoring, merge cases, data merging |
| [Organizations (B2B)](concepts/organizations.md) | Sub-organizations, which of them use CDD, and sharing schema attributes and unification rules down the tree |

## Guides

| Document | Description |
|---|---|
| [Installation](guides/on-prem-installation.md) | Installing, configuring and running CDD beside an Identity Server |
| [Searching Profiles](guides/searching-profiles.md) | Exact, fuzzy and hybrid search over an organization's profiles |
| [Reviewing Matches](guides/reviewing-matches.md) | The review queue: approving or rejecting the merges CDD holds back for a person |
| [Local Development Setup](guides/local-development.md) | Running CDD with an Identity Server locally, from source |
| [IS Sync](guides/is-sync.md) | Identity Server event integration — user lifecycle and session events |
| [Schema Sync](guides/schema-sync.md) | Keeping CDD schema aligned with IS claim changes |
| [Service Credentials](guides/service-credentials.md) | Where the credentials CDD calls the Identity Server management APIs with come from, one pair per organization |
| [Extending Queue Providers](guides/extending-queue-providers.md) | Adding a new message queue provider (Kafka, RabbitMQ, SQS, etc.) |
| [Logging](guides/logging.md) | Application, access, and audit logs — configuration, record formats, audit events, and how to log from a component |

## API

The REST API is described in
[`api/customer-data-service.yaml`](../api/customer-data-service.yaml).
