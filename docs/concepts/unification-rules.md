# Unification Rules

**Unification rules** tell CDS when two separate profiles should be recognised as the same person and merged. Each rule specifies a profile attribute to match on — if two profiles share the same value for that attribute, they are candidates for merging.

---

## Upgrading from exact-only matching

Before typed matching, unification walked the active rules in priority order and merged two
profiles on the **first exact match**, whatever the other rules held. Organisations that
have only ever used that behaviour keep it after upgrading, with no configuration change:

- A rule stored without `attribute_type`, `unification_method` or strengths is read as
  `PRIMITIVE_EXACT` + `deterministic`, matched on exact equality exactly as before.
- `PRIMITIVE_EXACT` carries `match_strength: HIGH`, so a single rule matching exactly still
  merges on its own, at any priority, however many other rules are configured.
- Automatic merging is on unless an organisation has explicitly turned it off. The
  `auto_merge_enabled` setting is stored as a row in the admin config, so an organisation
  configured before the key existed simply has no row for it; that absence reads as
  **enabled**, because reading it as disabled would silently route every merge the tenant
  relied on into the review queue instead.

Two behaviours are genuinely new for an existing organisation, and both only ever make the
engine *more* cautious:

- If several rules apply to a pair and most of them actively disagree, an automatic merge is
  downgraded to a review task rather than performed.
- If a rule is typed as `DATE` or `UNIQUE_ID` and the two values differ, that disagreement
  vetoes an automatic merge.

Neither can fire while every rule is an untyped legacy rule, because nothing is typed as
`DATE` or `UNIQUE_ID` and a lone exact match has nothing to disagree with it. They begin to
apply as an operator gives attributes their real types, which is the point at which they
want the extra caution.

---

## Rule structure

| Field | Description |
|---|---|
| `rule_id` | System-generated UUID |
| `org_handle` | The organisation this rule belongs to |
| `rule_name` | Human-readable name (also used as the `reason` recorded on a merge) |
| `property_name` | The attribute name to match on (e.g. `identity_attributes.email`) |
| `property_id` | The `attribute_id` of the schema attribute being matched |
| `priority` | Lower number = evaluated first. Rules are sorted ascending by priority. |
| `is_active` | Only active rules are evaluated during unification |
| `attribute_type` | What kind of value the attribute holds, which decides how it is compared and indexed |
| `unification_method` | `deterministic` (exact) or `fuzzy` (tolerates variation). Rules of both kinds are evaluated side by side. |
| `match_strength` | How much an agreement on this attribute supports a merge |
| `mismatch_strength` | How much a disagreement opposes one |
| `created_at` / `updated_at` | Timestamps |

---

## Choosing the evidence strengths

`match_strength` and `mismatch_strength` are asked for separately because agreement and
disagreement on the same attribute rarely carry equal weight:

- Two profiles sharing an **email address** are almost certainly the same person, but two
  *different* addresses say very little — most people have several.
- Two profiles sharing a **date of birth** says little, since birthdays collide constantly,
  yet two *different* dates is close to proof they are different people.

| Value | On a match | On a mismatch |
|---|---|---|
| `HIGH` | Can merge two profiles on its own | Two differing values block an automatic merge outright |
| `MEDIUM` | Real evidence, but another attribute must also agree | Counts against the match without blocking it |
| `LOW` | Close to coincidence on its own | Says little; people legitimately have several |

Both are derived from `attribute_type` using the table below, so a rule created without
thinking about strengths still behaves sensibly.

Setting them per rule is gated on `identity_resolution.allow_evidence_strength_override` in
`deployment.yaml`, which is **off by default**. While it is off, sending either field is
rejected rather than quietly ignored — a caller that believes it set a strength and is
overruled would misread every merge decision that followed. Turn it on only where someone
can judge the effect on existing profiles.

> **Disclaimer — this setting's scope is provisional.** It currently sits at deployment
> level because it gates an API surface rather than matching behaviour: whether a field is
> writable is a property of the build being run. Every other setting that shapes who gets
> merged — `auto_merge_enabled` and both thresholds — is per organisation in the admin
> config, so this may move there once there is a way for an operator to preview what a
> strength change would do to their existing profiles. Treat its location as unsettled and
> avoid building tooling that assumes it is server-wide.

`PRIMITIVE_EXACT` is what a rule written before typed matching resolves to, and it is
treated as strong in both directions on purpose: previously any rule matching exactly merged
the two profiles outright, and an upgrade must not quietly change that. An operator who
wants a weaker reading gives the attribute its real type.

| Attribute type | `match_strength` | `mismatch_strength` |
|---|---|---|
| `UNIQUE_ID` | HIGH | HIGH |
| `EMAIL` | HIGH | LOW |
| `PHONE` | HIGH | LOW |
| `DATE` | LOW | HIGH |
| `NAME` | LOW | MEDIUM |
| `LOCATION` | LOW | LOW |
| `FUZZY_STRING` | MEDIUM | LOW |
| `PRIMITIVE_EXACT` | HIGH | MEDIUM |

The strengths are returned on the rule so a client can display what the engine is applying,
even where they cannot be edited.

---

## How rules are evaluated

When a profile is created or updated it is enqueued for unification. The worker:

1. Fetches all active rules for the org, sorted by `priority` ascending
2. Fetches all existing master profiles for the org (excluding the current profile's own parent)
3. For each rule, checks whether any existing master profile has the same value for `property_name` as the incoming profile
4. On the first match, merges the two profiles and stops — only one rule fires per unification run

Rules are evaluated **after** the system-level `userId` match. If two profiles share the same `userId`, they are always merged regardless of any rules.

See [how-unification-works.md](how-unification-works.md) for the full merge pipeline.

---

## System merge reason

In addition to user-defined rules there is one built-in merge trigger:

| Reason | Trigger |
|---|---|
| `system:user_id_match` | Two profiles share the same `userId` — merged automatically without any rule |

This reason appears in `merged_from[].reason` on the master profile after the merge.

---

## Priority guidance

- Assign lower priority numbers to high-confidence identifiers (e.g. `identity_attributes.email` at priority 1)
- Assign higher priority numbers to weaker signals (e.g. `traits.phone` at priority 10)
- Leave gaps between priorities (e.g. 10, 20, 30) so new rules can be inserted without reordering

---

## Enabling and disabling rules

Setting `is_active: false` on a rule excludes it from evaluation without deleting it. Existing merges already recorded are not reversed when a rule is deactivated.
