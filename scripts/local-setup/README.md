# Local setup

```bash
./scripts/local-setup/script.sh up       # provision and start CDS + Identity Server
./scripts/local-setup/script.sh start    # start an already-provisioned work directory
./scripts/local-setup/script.sh --help   # all options
```

[docs/guides/local-development.md](../../docs/guides/local-development.md)
covers what the setup does and how to use it. This file covers the directory
layout.

## Layout

| | |
|---|---|
| `script.sh` | entry point: arguments, ordering, process control, smoke tests |
| `templates/` | the configuration that is generated, one file per artifact |
| `lib/` | the scripts that render and patch it |

Both directories have to be next to `script.sh`; preflight lists anything
missing.

## templates/

| File | Written to |
|---|---|
| `is-deployment.toml` | appended to the pack's `repository/conf/deployment.toml`, between `# BEGIN/END cds-local-dev` markers so that a re-run replaces it |
| `cds-deployment.yaml` | merged onto `config/repository/conf/deployment.yaml` to produce `<work-dir>/cds-home/repository/conf/deployment.yaml` |
| `console-features.json` | the Console's `deployment.config.json.j2`, on packs whose bundled Console predates the `cds_host` key |
| `openssl.cnf` | the request the CDS certificate is generated from |

Values are `${NAME}` placeholders, filled in at render time; a placeholder with
no value is an error. In `cds-deployment.yaml`, `${int:NAME}` produces a number
rather than a string.

Change what the setup configures by editing the template, not `script.sh`.

## lib/

| File | |
|---|---|
| `render.py` | fills in `${NAME}` placeholders: `render.py TEMPLATE NAME=VALUE ...` |
| `patch_is_toml.py` | removes a previously generated `deployment.toml` block; sets `offset` in the existing `[server]` table |
| `patch_console_template.py` | adds `cdsHost` and the feature blocks to an older Console template |
| `render_cds_config.py` | merges the overlay onto the shipped CDS config |

Each runs on its own, which is the quickest way to check a template change
without a full `up`.
