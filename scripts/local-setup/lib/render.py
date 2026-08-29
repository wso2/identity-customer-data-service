#!/usr/bin/env python3
"""Fill the ${NAME} placeholders in a template and print the result.

    render.py TEMPLATE NAME=VALUE ...

A placeholder with no value is an error rather than an empty string.
"""
import re, sys

PLACEHOLDER = re.compile(r"\$\{([A-Za-z_][A-Za-z_0-9]*)\}")


def main(argv):
    if len(argv) < 2:
        sys.exit(__doc__)
    path = argv[1]
    try:
        template = open(path).read()
    except OSError as exc:
        sys.exit("render.py: %s" % exc)

    values = {}
    for arg in argv[2:]:
        if "=" not in arg:
            sys.exit("render.py: expected NAME=VALUE, got %r" % arg)
        name, value = arg.split("=", 1)
        values[name] = value

    missing = sorted(set(PLACEHOLDER.findall(template)) - set(values))
    if missing:
        sys.exit("render.py: %s has no value for: %s" % (path, ", ".join(missing)))

    sys.stdout.write(PLACEHOLDER.sub(lambda m: values[m.group(1)], template))


if __name__ == "__main__":
    main(sys.argv)
