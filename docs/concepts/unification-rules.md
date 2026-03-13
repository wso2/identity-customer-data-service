# Unification Rules

**Unification rules** tell CDS when two separate profiles should be recognised as the same person and merged. Each rule specifies a profile attribute to match on — if two profiles share the same value for that attribute, they are candidates for merging.

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
| `created_at` / `updated_at` | Timestamps |

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
