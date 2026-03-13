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
                   └─ for each active rule (sorted by priority):
                         does any master profile share the same
                         value for rule.property_name?
                              │
                              yes ──► merge (rule.rule_name) ──► stop
                              no  ──► try next rule
```

---

## Step 1 — System userId match

If the incoming profile has a non-empty `userId`, CDS checks all existing master profiles for the same org. If any master profile has the same `userId`, the two profiles are merged immediately — no unification rule is required.

This is a system-level invariant. It fires before any rules are evaluated and cannot be disabled.

**Guard:** if both profiles are permanent (both have a `userId`) but with *different* user IDs, they are **not** merged.

---

## Step 2 — Rule-based matching

Active rules for the org are fetched, filtered, and sorted by `priority` ascending. For each rule CDS compares the value of `rule.property_name` across the incoming profile and all existing master profiles. The first rule that produces a match triggers a merge.

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
