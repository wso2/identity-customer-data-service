# How Profile Unification Works

Profile unification is the process of recognising that two separate profile records represent the same person and merging them into a single master profile. It runs asynchronously via a background worker queue.

---

## Overview

```
Profile created / updated
         │
         ▼
  Enqueued for unification (ProfileUnificationQueue)
         │
         ▼
  Worker picks up profile
         │
         ├─ Step 1: userId match? ──yes──► merge (system:user_id_match)
         │
         └─ Step 2: rule-based match?
                   │
                   ├─ block:  find candidates sharing an index key
                   ├─ score:  evaluate every rule against each candidate
                   └─ decide: AUTO_MERGE / MANUAL_REVIEW / UNIQUE
```

---

## Step 1 — System userId match

If the incoming profile has a non-empty `userId`, CDS checks all existing master profiles for the same org. If any master profile has the same `userId`, the two profiles are merged immediately — no unification rule is required.

This is a system-level invariant. It fires before any rules are evaluated and cannot be disabled.

**Guard:** if both profiles are permanent (both have a `userId`) but with *different* user IDs, they are **not** merged.

---

## Step 2 — Rule-based matching

Active rules are fetched, defaulted, and sorted by `priority` ascending. Candidates are found from the blocking index, then every candidate is scored against the full rule set and routed by the org's thresholds.

Deterministic and fuzzy rules are not alternative engines. A single comparison honours each rule's own `unification_method` — exact equality for `deterministic`, similarity for `fuzzy` — and aggregates them into one decision.

### What each rule reports

A rule produces a similarity score *and* a verdict. The verdict matters because a score alone cannot distinguish "these are different people" from "there was nothing to compare":

| Verdict | When | Effect on the decision |
|---|---|---|
| `AGREE` | score ≥ `manual_review_threshold` | Can become the primary signal; counts toward the agreement total |
| `INCONCLUSIVE` | score between the contradiction and agreement bars | Ignored in both directions |
| `DISAGREE` | score ≤ `ScoreContradictionThreshold` (0.3) | Counts as contradicting evidence |
| `UNKNOWN` | either side has no value, or holds a value that identifies nobody | Excluded entirely — never affects the score |

`UNKNOWN` covers placeholders as well as absent values: role mailboxes (`noreply@`, `info@`), reserved domains (`example.com`), sentinel dates (`1970-01-01`), repeated-digit phone numbers and `N/A`-style text. Two profiles agreeing on one of those agree on nothing.

### Evidence strength

Agreement and disagreement on the same attribute rarely carry equal weight, so each rule declares both directions independently:

| | `match_strength` | `mismatch_strength` |
|---|---|---|
| **Email** | HIGH — sharing an address strongly implies one person | LOW — most people have several addresses |
| **Date of birth** | LOW — birthdays collide constantly | HIGH — two different dates is near-proof of difference |
| **National ID** | HIGH | HIGH |
| **Name** | LOW | MEDIUM |

Both default from `attribute_type` when the operator does not set them. A rule with `mismatch_strength: HIGH` is *discriminating*: a disagreement on it vetoes automatic merging whatever else agrees.

### The algorithm

The score is **not an average**. Averaging made every rule's weight depend on how many other rules happened to be configured, so adding a rule weakened every existing match, and a rule that disagreed pulled a strong match down by exactly as much as a rule with no data — which meant two sparse profiles outscored two well-populated ones. Instead one rule speaks for the match and the rest may only object.

| Step | | |
|---|---|---|
| 1 | **Evaluate** | Score every rule in its own mode. `UNKNOWN` rules take no further part. |
| 2 | **Primary signal** | Walk rules in priority order; the first that `AGREE`s is the primary. Its score is the result — lower-priority rules cannot dilute it. |
| 3 | **Unique-ID short-circuit** | A `UNIQUE_ID` agreeing exactly returns 1.0 immediately. |
| 4 | **Discriminating veto** | Any `mismatch_strength: HIGH` rule that `DISAGREE`s caps the score just below auto-merge. |
| 5 | **Conflicting evidence** | If most other applicable rules `DISAGREE`, cap below auto-merge. |
| 6 | **Lone-agreement gate** | A single agreeing rule may auto-merge only if it is `match_strength: HIGH` or the top-priority rule, **and** the matched value is not already shared by ≥ `RarityCommonMinProfiles` profiles in the org. |

Caps only ever downgrade `AUTO_MERGE` to `MANUAL_REVIEW`. They never suppress a match to `UNIQUE` — the primary signal still stands. If no rule agrees, the highest applicable score is returned, which is below the review threshold by construction.

**Value rarity** (step 6) is read from the `blocking_keys` index: how many profiles in the org already carry the matched value. Rarity may only weaken evidence, never strengthen it — an uncommon value being wrong is just as wrong as a common one, so rarity lowers the chance of coincidence without adding corroboration.

### Decision thresholds

| Score | Decision |
|---|---|
| ≥ `auto_merge_threshold` (default 0.95) | `AUTO_MERGE` |
| ≥ `manual_review_threshold` (default 0.75) | `MANUAL_REVIEW` — a review task is created |
| below that | `UNIQUE` — no action |

`AUTO_MERGE` additionally requires `auto_merge_enabled` on the org's admin config.

### Worked examples

Rules: **P1** `nic` (UNIQUE_ID, deterministic) · **P2** `name` (NAME, fuzzy) · **P3** `city` (LOCATION, fuzzy). Thresholds 0.95 / 0.75.

| # | Scenario | nic | name | city | Outcome | Why |
|---|---|---|---|---|---|---|
| 1 | All agree | 1.00 | 0.97 | 1.00 | **AUTO_MERGE** | Unique ID agrees exactly — short-circuit |
| 2 | NIC differs, name close | 0.00 | 0.97 | 1.00 | **MANUAL_REVIEW** | Name is primary; NIC is discriminating and disagrees → veto |
| 3 | NIC absent on one side | — | 0.97 | 1.00 | **AUTO_MERGE** | Name and city both agree — two independent agreements |
| 4 | NIC absent, only name | — | 0.97 | — | **MANUAL_REVIEW** | Lone agreement on a LOW-strength attribute |
| 5 | Only city shared | — | — | 1.00 | **MANUAL_REVIEW** | Lone agreement, not top priority, weak attribute |
| 6 | NIC agrees, everything else differs | 1.00 | 0.10 | 0.00 | **AUTO_MERGE** | Unique ID is conclusive on its own |
| 7 | Nothing reaches the bar | 0.00 | 0.42 | 0.30 | **UNIQUE** | No primary signal |

Rules: **P1** `email` (EMAIL, deterministic) · **P2** `dob` (DATE, deterministic).

| # | Scenario | email | dob | Outcome | Why |
|---|---|---|---|---|---|
| 8 | Email agrees, DOB absent | 1.00 | — | **AUTO_MERGE** | Email is `match_strength: HIGH` and top priority |
| 9 | Email agrees, DOB differs | 1.00 | 0.00 | **MANUAL_REVIEW** | DOB is `mismatch_strength: HIGH` → veto |
| 10 | Email agrees but 300 profiles share it | 1.00 | — | **MANUAL_REVIEW** | Value too common to identify anyone |
| 11 | Both hold `noreply@example.com` | — | — | **UNIQUE** | Placeholder reads as `UNKNOWN`, not as agreement |

### Why the previous model was replaced

With P1 `nic` deterministic and P2 `name` fuzzy, weights 2 and 1, the old weighted average divided by the total applicable weight:

```
NIC differs, name matches 0.95:   (0.0 × 2 + 0.95 × 1) / 3  =  0.32  →  UNIQUE
NIC absent,  name matches 0.95:   (0.95 × 1) / 1            =  0.95  →  AUTO_MERGE
```

The same name evidence produced no action when the NICs contradicted, and an automatic merge when the NIC was simply missing — profiles merged more readily the less was known about them. Both rows are now `MANUAL_REVIEW`.

---

## Merge cases

Once a match is found, the worker determines the master/child relationship based on whether each profile is *permanent* (has a `userId`) or *temporary* (anonymous, no `userId`).

### Permanent + temporary

The permanent profile always becomes the master. The temporary profile is added as a child reference.

```
existing: permanent  +  new: temporary
  → existing stays as master
  → new becomes child of existing
```

```
existing: temporary  +  new: permanent
  → new becomes master
  → existing becomes child of new
  → existing's children (if any) are re-parented to new
```

### Both temporary

The existing profile becomes master, the new profile becomes its child.

### Both permanent (same userId)

Treated the same as both-temporary — existing becomes master, new becomes child.

### Both permanent (different userIds)

**Not merged.** CDS logs a warning and returns without action.

---

## Data merging

After the master/child relationship is resolved, `MergeProfiles` is called. It walks each schema attribute for the org and applies the attribute's `merge_strategy`:

| Strategy | Behaviour |
|---|---|
| `overwrite` | Incoming value replaces existing |
| `combine` | Both values combined into an array |

The merged result is written back to the master profile across three stores: `identity_attributes`, `traits`, and `application_data`.

---

## Result on the profile

After a merge the API response reflects:

- **Master profile** — `merged_from` contains a `Reference` entry for each child profile with the rule name (or `system:user_id_match`) as the `reason`
- **Child profile** — `merged_to` points back to the master profile's `profile_id`

```json
// Master
{
  "profile_id": "master-123",
  "user_id": "alice@example.com",
  "merged_from": [
    { "profile_id": "anon-456", "reason": "system:user_id_match" },
    { "profile_id": "old-789",  "reason": "email_match" }
  ]
}

// Child
{
  "profile_id": "anon-456",
  "merged_to": { "profile_id": "master-123", "reason": "system:user_id_match" }
}
```
