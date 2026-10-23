# Update tool onboarding

An installed pack needs two things before the WSO2 update tool can update it
in place, and they come from two different places:

| In the pack | Produced by |
|---|---|
| `updates/product.txt` | `make dist`, from `PRODUCT_NAME` and the version |
| `bin/update_tool_setup.sh` | the Updates team, attached to the released packs |

This split is not ours to choose — it is how WSO2 products are already built.
`product-is` keeps `updates/product.txt` in its source tree and packs it with
the distribution; the setup script appears in nothing it builds, and is added
when the pack is published. CDS follows that, so the product build stays
buildable from a public checkout with nothing else present.

## `updates/product.txt`

One line naming the pack to the update service:

```
cds-linux-amd64-1.2.0
```

The pack directory, its zip and this identity are all the same string, which
is why each platform is a separate product. `make dist` writes it; the name
has to be registered with the update service before updates resolve, so it is
not a string this repository can choose freely.

## `bin/update_tool_setup.sh`

Handed over by the Updates team as `update_tool_setup.zip`. It is not vendored
as a loose file, because that zip is the unit they publish and sign.

The script contains no update binary. It detects the host operating system and
CPU architecture, asks the update service which platforms the product
supports, and downloads the matching `wso2update_<platform>`. That is why no
update binary is bundled, and why one script serves every pack. It resolves
`../updates/product.txt` relative to its own location, so `bin/` and
`updates/` must stay siblings at the pack root.

[`attach-update-tool.sh`](attach-update-tool.sh) installs it into built packs
and rewrites their checksums:

```bash
make dist
distribution/u2-onboarding/attach-update-tool.sh target/dist update_tool_setup.zip
```

The `🔄 Attach Update Tool` workflow does the same thing to the assets of a
published release, so the onboarding step does not depend on anyone's working
copy.

## Why not in `make dist`

Because the product build would then depend on a file that is not in the
repository, and would either fail without it or silently produce packs that
cannot be updated. Keeping it out means `make dist` has one behaviour for
everyone, and the one step that needs the Updates team's artifact is the step
that is run by the people who have it.
