#!/usr/bin/env python3
"""Teach an older bundled Console about CDS.

    patch_console_template.py J2_TEMPLATE FEATURES_JSON CDS_HOST

The Console resolves its CDS endpoints from `deploymentConfig.extensions.cdsHost`
and gates the Customer Data section on four `customerData*` features. A current
pack renders both from deployment.toml, so this runs only as a fallback, when
the pack's deployment.config.json.j2 has no `cds_host` key at all: it inserts
the cdsHost extension and the feature blocks straight into the template.

Idempotent - a template that already carries cdsHost is left alone.
"""
import json, re, sys


def main(path, features_path, cds_host):
    src = open(path).read()
    if "cdsHost" in src:
        print("    already patched")
        return 0

    features = {k: v for k, v in json.load(open(features_path)).items()
                if not k.startswith("_")}

    # 1. cdsHost inside the "extensions" object.
    m = re.search(r'"extensions"\s*:\s*\{', src)
    if not m:
        sys.exit('could not find the "extensions" block in %s' % path)
    src = src[:m.end()] + '\n        "cdsHost": "%s",' % cds_host + src[m.end():]

    # 2. The customerData feature blocks inside "features".
    def block(name, scopes):
        return json.dumps({
            "disabledFeatures": [],
            "enabled": True,
            "featureFlags": [{"feature": name, "flag": ""}],
            "scopes": scopes,
        }, indent=4)

    m = re.search(r'"features"\s*:\s*\{', src)
    if not m:
        sys.exit('could not find the "features" block in %s' % path)
    inserted = "\n" + ",\n".join(
        '            "%s": %s' % (name, block(name, scopes))
        for name, scopes in features.items()
    ) + ","
    src = src[:m.end()] + inserted + src[m.end():]

    open(path, "w").write(src)
    print("    patched deployment.config.json.j2 (cdsHost + %d feature blocks)" % len(features))
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 4:
        sys.exit(__doc__)
    sys.exit(main(sys.argv[1], sys.argv[2], sys.argv[3]))
