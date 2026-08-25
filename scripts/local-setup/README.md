# Local setup

```bash
./scripts/local-setup/script.sh up       # provision and start CDS + Identity Server
./scripts/local-setup/script.sh start    # bring the same setup back up, no setup steps
./scripts/local-setup/script.sh --help   # every option
```

See [../../docs/guides/local-development.md](../../docs/guides/local-development.md)
for what it does and how to use it. This file is about how the directory is put
together.

## Layout

| | |
|---|---|
| `script.sh` | the entry point — argument handling, ordering, process control, smoke tests |
| `templates/` | the configuration it writes, one file per artifact |
| `lib/` | the helpers that render and patch that configuration |

`script.sh` needs both directories next to it; it checks for them during
preflight and stops with the list of anything missing.

## templates/

| File | Written to |
|---|---|
| `is-deployment.toml` | appended to the pack's `repository/conf/deployment.toml`, between `# BEGIN/END cds-local-dev` markers so a re-run replaces it instead of adding a second copy |
| `cds-deployment.yaml` | merged onto the repository's `config/repository/conf/deployment.yaml` to produce `<work-dir>/cds-home/repository/conf/deployment.yaml` |
| `console-features.json` | the Console's `deployment.config.json.j2` — only on packs whose bundled Console predates the `cds_host` key |
| `openssl.cnf` | the request the CDS self-signed certificate is generated from |

Values are filled in as `${NAME}` placeholders. Nothing is optional: rendering
fails if a placeholder has no value, so a half-filled configuration file cannot
reach a server. In `cds-deployment.yaml`, `${int:NAME}` produces a number rather
than a string, matching how the shipped file types those keys.

To change what the setup configures, edit the template — not `script.sh`.

## lib/

| File | |
|---|---|
| `render.py` | fills in `${NAME}` placeholders; `render.py TEMPLATE NAME=VALUE ...` |
| `patch_is_toml.py` | removes a previously generated `deployment.toml` block, and sets `offset` inside the existing `[server]` table |
| `patch_console_template.py` | the Console compatibility fallback described above |
| `render_cds_config.py` | merges the overlay onto the shipped CDS config and writes the result |

Each is runnable on its own, which is the easy way to try a template change
without a full `up`.
