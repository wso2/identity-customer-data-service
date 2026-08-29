#!/usr/bin/env python3
"""Edit an Identity Server deployment.toml in place.

    patch_is_toml.py strip  TOML BEGIN_MARKER END_MARKER
    patch_is_toml.py offset TOML N

`strip` removes a previously generated block, matching the markers by prefix so
that a reworded marker still finds an older block. `offset` sets offset inside
the existing [server] table; a second [server] table would be a duplicate key.
"""
import re, sys


def strip(path, begin, end):
    out, skipping = [], False
    for line in open(path).read().split("\n"):
        stripped = line.strip()
        if stripped.startswith(begin):
            skipping = True
            continue
        if skipping:
            if stripped.startswith(end):
                skipping = False
            continue
        out.append(line)
    open(path, "w").write("\n".join(out).rstrip("\n") + "\n")


def offset(path, value):
    lines = open(path).read().split("\n")
    start = next((i for i, l in enumerate(lines) if l.strip() == "[server]"), None)
    if start is None:
        if value:
            lines[:0] = ["[server]", "offset = %d" % value, ""]
            open(path, "w").write("\n".join(lines))
        return

    # Only the [server] table: another table may have an offset of its own.
    end = next((i for i in range(start + 1, len(lines))
                if lines[i].lstrip().startswith("[")), len(lines))
    body = [l for l in lines[start + 1:end] if not re.match(r"\s*offset\s*=", l)]
    if value:
        body.insert(0, "offset = %d" % value)
    lines[start + 1:end] = body
    open(path, "w").write("\n".join(lines))


if __name__ == "__main__":
    if len(sys.argv) >= 5 and sys.argv[1] == "strip":
        strip(sys.argv[2], sys.argv[3], sys.argv[4])
    elif len(sys.argv) == 4 and sys.argv[1] == "offset":
        offset(sys.argv[2], int(sys.argv[3]))
    else:
        sys.exit(__doc__)
