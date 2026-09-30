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
- The strength columns are nullable with **no column default**. A default would stamp a
  concrete value onto every pre-existing row when the column is added, which is
  indistinguishable from an operator having chosen it and would override what the attribute
  type implies. An unset strength is derived on every read instead, so changing a rule's
  `attribute_type` later re-derives it rather than leaving the old type's value behind.
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

A profile write is enqueued for unification when any of the following is true. An update
that changed nothing a rule matches on is skipped, because it would reach the conclusion the
previous write already reached:

- a value some active rule matches on changed
- the profile's `userId` changed
- an active rule is newer than the profile's last write (see below)

The worker then:

1. Merges immediately if another master profile shares the same `userId` — a system
   invariant that needs no rule
2. Regenerates the profile's blocking keys from the active rules, replacing its previous set
3. Asks the index which other profiles share a key, capped per key group
4. Scores each of those candidates against the **whole** rule set
5. Routes the result by the org's thresholds: merge, raise a review task, or do nothing

Every rule is evaluated against every candidate — a rule does not "fire" on its own. One
rule then speaks for the result and the others may only object; see
[how-unification-works.md](how-unification-works.md) for that algorithm.

---

## When a rule starts applying

A rule does not only affect profiles written after it. There are three paths by which an
existing profile comes under a new rule, and they exist because no single one is sufficient:

| Path | Covers | Limitation |
|---|---|---|
| The profile's next write | Any profile that is written again | Not immediate; never happens for a dormant profile |
| A write while the rule is newer than the profile | An unrelated edit still brings the profile in | Still needs the profile to be written |
| The backfill on activation | Every existing profile, immediately | Runs in the background; a restart mid-scan leaves it partial |

The backfill is the only one that is both immediate and complete, which is why activating a
rule triggers it. The other two are what make the index self-healing for anything it missed.

---

## Rejections

Rejecting a review task records that an administrator decided two profiles are **different
people**, along with the match score and per-rule breakdown they decided against.

That evidence is what makes the decision durable. A rejection is a statement about identity,
not about the data at the time — two different people do not become the same person because
one of them changed a phone number — so an attribute changing is not by itself a reason to
ask again. Clearing rejections on data change turns the review queue into a treadmill: the
same pair returns whenever anything unrelated moves, and the administrator dismisses it
repeatedly.

A rejected pair is put back in front of an administrator only when the new evaluation is
genuinely stronger than the one they saw:

| Reopens the pair | Leaves the rejection standing |
|---|---|
| The score exceeds the rejected score by `RejectionReconsiderMargin` (0.05) | The score is the same, lower, or drifts up slightly |
| A rule agrees now that did not agree then — including an attribute that was not comparable before | A new attribute appears but does not agree |

The second row of the left column matters because of how scoring works: the waterfall takes
its score from the first rule that agrees, so a *lower-priority* rule newly agreeing adds
real corroboration without moving the number at all.

Rejections follow their profiles. A merge repoints a rejection from the merged-away profile
onto the surviving master, and deleting a profile removes its rejections.

---

## System merge reason

In addition to user-defined rules there is one built-in merge trigger:

| Reason | Trigger |
|---|---|
| `system:user_id_match` | Two profiles share the same `userId` — merged automatically without any rule |

This reason appears in `merged_from[].reason` on the master profile after the merge.

---

## Priority guidance

Priority is a **gate**, not a weight. Rules are walked in priority order and the first one
that agrees sets the match score outright; lower-priority rules corroborate it but cannot
raise it. Ordering therefore decides which rule is held accountable for a merge, which is
what an operator sees when asking why two profiles were combined.

- Assign lower priority numbers to high-confidence identifiers (e.g. `identity_attributes.email` at priority 1)
- Assign higher priority numbers to weaker signals (e.g. `traits.city` at priority 10)
- Leave gaps between priorities (e.g. 10, 20, 30) so new rules can be inserted without reordering
- Priorities must be unique within an organisation; a duplicate is rejected

A lone agreement on a rule that is neither the top-priority one nor marked strong evidence
cannot auto-merge on its own — it is capped to a review task.

---

## Enabling and disabling rules

Setting `is_active: false` excludes a rule from evaluation without deleting it, and removes
that attribute's blocking keys for the organisation — the index shrinks rather than carrying
entries nothing reads. Re-activating it starts the backfill again.

Changing `attribute_type` on an active rule does both: the old keys are removed and rebuilt,
because the key shape a type produces is different.

Existing merges already recorded are not reversed when a rule is deactivated or deleted.
