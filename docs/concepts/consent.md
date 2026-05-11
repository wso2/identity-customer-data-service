# Consent

Consent controls which profile attributes a caller is allowed to **read**. When an app fetches a profile, the response is filtered to only the attributes the user has consented to under the declared consent categories.

---

## Consent categories

A **consent category** is an org-level definition that declares:

- A human-readable name and a stable auto-generated UUID identifier
- The **purpose** of data use (`profiling`, `personalization`, or `destination`)
- The **attributes** it covers — which profile fields an app is allowed to see when operating under this consent
- Whether it is **mandatory** — mandatory categories are always enforced and cannot be modified or deleted

**Create / update request format:**

```json
{
  "category_name": "Product Engagement",
  "purpose": "profiling",
  "destinations": ["segment"],
  "attributes": [
    { "attribute_name": "traits.engagement_score" },
    { "attribute_name": "traits.product_interests" },
    { "attribute_name": "application_data.events.event_name",      "application_identifier": "i0QfDlYH6BIA8QygvmLRHikFrRIa" },
    { "attribute_name": "application_data.events.event_year",      "application_identifier": "i0QfDlYH6BIA8QygvmLRHikFrRIa" },
    { "attribute_name": "application_data.saas_signups.signup_id", "application_identifier": "i0QfDlYH6BIA8QygvmLRHikFrRIa" }
  ]
}
```

All attributes — `traits`, `identity_attributes`, and `application_data` — are listed under `"attributes"` as objects with `attribute_name` (required) and an optional `application_identifier`. `application_identifier` is only required for `application_data.*` attributes and must match the `application_identifier` registered in the org's profile schema.

`category_identifier` is always server-generated — any value supplied by the caller is ignored.

**GET response format** (after creation):

```json
{
  "category_name": "Product Engagement",
  "category_identifier": "fb5b2bd7-...",
  "purpose": "profiling",
  "is_mandatory": false,
  "attributes": [
    { "scope": "traits",           "attribute_name": "traits.engagement_score" },
    { "scope": "traits",           "attribute_name": "traits.product_interests" },
    { "scope": "applicationData",  "attribute_name": "application_data.events.event_name",      "application_identifier": "i0QfDlYH6BIA8QygvmLRHikFrRIa" },
    { "scope": "applicationData",  "attribute_name": "application_data.events.event_year",      "application_identifier": "i0QfDlYH6BIA8QygvmLRHikFrRIa" },
    { "scope": "applicationData",  "attribute_name": "application_data.saas_signups.signup_id", "application_identifier": "i0QfDlYH6BIA8QygvmLRHikFrRIa" }
  ]
}
```

**Validation at write time:**

- Every `attribute_name` is looked up in the org's `profile_schema`. If not found the request is rejected with `400`.
- `scope` is derived automatically from the attribute name prefix (`traits.*`, `identity_attributes.*`, `application_data.*`) — never supplied by the caller.
- For `applicationData` attributes, `application_identifier` is required and **must match** the `application_identifier` stored in `profile_schema` for that attribute. A missing or mismatched value is rejected with `400`.
- Each attribute's `attribute_id` (FK → `profile_schema`) is resolved at write time and stored, enabling `ON DELETE CASCADE` when a schema attribute is removed.

### Attribute scopes (derived, not supplied)

| Scope | Description |
|---|---|
| `identityAttributes` | Core identity fields (email, phone, name, etc.) |
| `traits` | Behavioural and preference fields |
| `applicationData` | Per-application data. `application_identifier` (the app's OAuth client ID) must match the schema's `application_identifier` |

### applicationData attributes and application_identifier

A profile's `application_data` is a two-level map:

```
application_data
  └── <app_client_id>          ← outer key: the app's OAuth client ID
        └── <data_group_key>   ← inner key: logical data category (e.g. "events", "saas_signups")
              └── <value>
```

When declaring consent attributes for application data, `application_identifier` is the **outer app client ID**. The attribute name encodes the inner data-group key: `application_data.<data_group>.<field>`.

At filter time, consent is enforced by matching the **inner data-group key** against the consented attribute names — not the outer app client ID. This means if multiple apps write data under the same data-group key (e.g. `"events"`), they are all gated by the same consent entry.

### Complex attributes

If an attribute has sub-attributes (e.g. `address` with `city`, `zip`), adding the parent `attribute_name` to the consent category is sufficient. The entire object is returned as-is — sub-attributes do not need to be listed individually.

The same applies to `applicationData`: adding `application_data.events.event_name` (a sub-attribute of `events`) allows the entire `events` value in the app's data to be returned, since array entries cannot be partially filtered.

### Consent granularity

Consent is tracked at the **category level**, not the attribute level. A profile either consents to an entire category or not — there is no way to consent to some attributes within a category and not others.

This is by design. If finer-grained control is needed, define more granular categories. For example, instead of a single "Marketing" category covering contact details, preferences, and purchase history, define separate "Marketing - Contact" and "Marketing - Preferences" categories so users can consent to each independently.

---

## Mandatory consent — Identity Data

Every org has a built-in consent category called **Identity Data** that is automatically created when CDS is enabled. It covers all `identityAttributes` scope fields defined in the org's profile schema. Its `category_identifier` is a server-generated UUID.

```json
{
  "category_name": "Identity Data",
  "category_identifier": "<server-generated UUID>",
  "purpose": "profiling",
  "is_mandatory": true
}
```

**Properties of mandatory categories:**

- Always included in consent filtering — no `profile_consents` record is required
- Cannot be deleted or updated via the API (`403 Forbidden`)
- Cannot have consent revoked per profile (`403 Forbidden`)
- Visible via `GET /consent-categories` with `"is_mandatory": true`

**Attributes are resolved dynamically.** The Identity Data category does not store rows in `consent_category_attributes`. At filter time, the system queries `profile_schema WHERE scope = 'identity_attributes'` live. This means adding or removing an identity attribute via schema sync is automatically reflected in consent filtering — no reseeding or migration is needed.

---

## Per-profile consent records

Each profile has its own consent status per category, stored in `profile_consents`:

```
profile_consents
  ├── profile_id      → the profile
  ├── category_id     → consent_categories.category_identifier
  ├── consent_status  → true (consented) | false (revoked)
  └── consented_at    → timestamp of last change
```

`UNIQUE (profile_id, category_id)` — one record per profile per category.

Managed via:

```
GET  /api/v1/{orgHandle}/profiles/{profileId}/consents
PUT  /api/v1/{orgHandle}/profiles/{profileId}/consents
```

The `PUT` replaces the full consent record set for the profile. Mandatory categories cannot be included in the payload — attempting to do so returns `403`.

### Example — reading a profile's consents

```
GET /api/v1/acme/profiles/p-001/consents
```

```json
[
  { "category_identifier": "<identity-data UUID>", "is_consented": true,  "consented_at": "2026-01-10T08:00:00Z" },
  { "category_identifier": "<marketing UUID>",     "is_consented": true,  "consented_at": "2026-01-10T08:00:00Z" },
  { "category_identifier": "<analytics UUID>",     "is_consented": false, "consented_at": "2026-02-01T14:30:00Z" }
]
```

> The mandatory Identity Data category always appears as consented — it cannot be revoked.

### Example — updating a profile's consents

```
PUT /api/v1/acme/profiles/p-001/consents
```

```json
[
  { "category_identifier": "<marketing UUID>", "is_consented": true  },
  { "category_identifier": "<analytics UUID>", "is_consented": false }
]
```

This replaces all non-mandatory consent records. Mandatory categories must not be included.

---

## Consent-scoped profile fetch

When fetching a profile, third-party apps always receive a consent-filtered response. System apps (registered in `system_applications`) bypass consent filtering and always receive the full profile.

```
GET /api/v1/{orgHandle}/profiles/{profileId}
GET /api/v1/{orgHandle}/profiles/{profileId}?consentCategoryId=<marketing UUID>
GET /api/v1/{orgHandle}/profiles/{profileId}?consentCategoryId=<marketing UUID>&consentCategoryId=<analytics UUID>
GET /api/v1/{orgHandle}/profiles/{profileId}?consentCategoryId=<marketing UUID>&includeApplicationData=true
```

`application_data` is **excluded by default**. Pass `includeApplicationData=true` to include it. When included, regular apps only receive data scoped to their own `application_identifier`; system apps receive all app data (or can filter by `application_identifier`).

### Behaviour by caller and consentCategoryId

| Caller | `consentCategoryId` passed | `includeApplicationData` | Response |
|---|---|---|---|
| System app | yes or no | any | Full profile — consent filtering bypassed |
| Regular app | no | any | Mandatory identity data fields only |
| Regular app | yes, user has consented | `false` (default) | Identity data fields + traits from consented categories |
| Regular app | yes, user has consented | `true` | Identity data fields + traits + caller's own `application_data` from consented categories |
| Regular app | yes, user has NOT consented | any | Identity data fields only (non-consented categories contribute nothing) |
| Regular app | multiple consentCategoryIds | `true` | Identity data fields + **union** of traits and `application_data` across all consented categories |

---

## Example — full consent flow for a single profile

### Setup

The org has three consent categories:

| Identifier | Attributes |
|---|---|
| Identity Data *(mandatory)* | `email`, `phone`, `username` |
| Marketing | `email`, `phone`, `age`, `last_purchase` (app: crm) |
| Analytics | `email`, `age`, `last_login` |

Profile `p-001` has the following consent records:

| Category | Status |
|---|---|
| Marketing | consented |
| Analytics | revoked |

---

### Case 1 — No consentCategoryId (regular app)

```
GET /api/v1/acme/profiles/p-001
```

No `consentCategoryId` passed → mandatory Identity Data fields only.

```json
{
  "profile_id": "p-001",
  "user_id": "u-123",
  "identity_attributes": {
    "email": "jane@example.com",
    "phone": "+1-555-0100",
    "username": "jane"
  }
}
```

---

### Case 2 — Single consentCategoryId, user has consented

```
GET /api/v1/acme/profiles/p-001?consentCategoryId=<marketing UUID>&includeApplicationData=true
```

Marketing is consented → union of Identity Data + Marketing attributes. `application_data` is included because `includeApplicationData=true` is set; the caller's own app data (`com.acme.crm`) is returned.

```json
{
  "profile_id": "p-001",
  "user_id": "u-123",
  "identity_attributes": {
    "email": "jane@example.com",
    "phone": "+1-555-0100",
    "username": "jane"
  },
  "traits": {
    "age": 32
  },
  "application_data": {
    "com.acme.crm": {
      "last_purchase": "2026-01-05"
    }
  }
}
```

---

### Case 3 — Single consentCategoryId, user has NOT consented

```
GET /api/v1/acme/profiles/p-001?consentCategoryId=<analytics UUID>
```

Analytics is revoked → only mandatory Identity Data fields returned.

```json
{
  "profile_id": "p-001",
  "user_id": "u-123",
  "identity_attributes": {
    "email": "jane@example.com",
    "phone": "+1-555-0100",
    "username": "jane"
  }
}
```

---

### Case 4 — Multiple consentCategoryIds (union)

```
GET /api/v1/acme/profiles/p-001?consentCategoryId=<marketing UUID>&consentCategoryId=<analytics UUID>&includeApplicationData=true
```

Marketing is consented, Analytics is revoked → union of Identity Data + Marketing only.

```json
{
  "profile_id": "p-001",
  "user_id": "u-123",
  "identity_attributes": {
    "email": "jane@example.com",
    "phone": "+1-555-0100",
    "username": "jane"
  },
  "traits": {
    "age": 32
  },
  "application_data": {
    "com.acme.crm": {
      "last_purchase": "2026-01-05"
    }
  }
}
```

> `last_login` from Analytics is absent because the user revoked that consent.

---

### Case 5 — System app (no filtering)

```
GET /api/v1/acme/profiles/p-001
X-App-Id: console-app   ← registered as a system application
```

System app → full profile regardless of consent.

```json
{
  "profile_id": "p-001",
  "user_id": "u-123",
  "identity_attributes": { "email": "jane@example.com", "phone": "+1-555-0100", "username": "jane" },
  "traits": { "age": 32, "last_login": "2026-03-20T10:00:00Z" },
  "application_data": {
    "com.acme.crm": { "last_purchase": "2026-01-05" }
  }
}
```

---

## Profile listing

`GET /profiles` does **not** apply consent filtering. Listing is an administrative operation and always returns full profiles.

---

## What consent does NOT affect

| Area | Behaviour |
|---|---|
| Profile **writes** | Consent does not gate write operations. Controlled by permissions. |
| Profile **listing** | No consent filtering — full profiles returned. |
| System app access | Consent filtering is bypassed entirely. |
| Mandatory category | Always contributes identity attributes — no profile consent record needed. |

---

## Data model

```
consent_categories
  ├── category_identifier  (unique UUID, always server-generated)
  ├── purpose              (profiling | personalization | destination)
  ├── is_mandatory         (true = system-managed, cannot be modified or deleted)
  └── consent_category_attributes  (not used for mandatory categories — see below)
        ├── attribute_name (references profile_schema.attribute_name)
        ├── attribute_id   (FK → profile_schema.attribute_id ON DELETE CASCADE)
        ├── scope          (derived from profile_schema at write time — not supplied by caller)
        └── application_identifier  (applicationData only — must match profile_schema.application_identifier)

profile_consents
  ├── profile_id           → profiles
  ├── category_id          → consent_categories
  ├── consent_status       (true = consented, false = revoked)
  └── consented_at
```

> **Mandatory categories** have no rows in `consent_category_attributes`. Their attributes are resolved live from `profile_schema WHERE scope = 'identity_attributes'` at filter time, so schema changes are reflected automatically.

> **Cascade deletion:** `consent_category_attributes.attribute_id` references `profile_schema(attribute_id) ON DELETE CASCADE`. When a schema attribute is deleted, its rows in `consent_category_attributes` are automatically removed by the database — no application-level cleanup is needed.

---

## Sequence — consent-filtered profile fetch

```
Caller                    API                  ConsentFilterService              DB
  |                        |                          |                           |
  | GET /profiles/{id}     |                          |                           |
  |  ?consentCategoryId=<uuid>     |                          |                           |
  |----------------------->|                          |                           |
  |                        |-- fetch raw profile ---->|                           |
  |                        |                          |-------------------------->|
  |                        |                          |<-- full profile ----------|
  |                        |                          |                           |
  |                        |-- filterByConsent() ---->|                           |
  |                        |   (profile, orgHandle,   |                           |
  |                        |    [<uuid>])             |                           |
  |                        |                          |-- profile_consents ------>|
  |                        |                          |   (has user consented?)   |
  |                        |                          |<-- consent rows ----------|
  |                        |                          |                           |
  |                        |                          |-- mandatory categories -->|
  |                        |                          |   (is_mandatory = true)   |
  |                        |                          |<-- mandatory ids ---------|
  |                        |                          |                           |
  |                        |                          |-- category_attributes --->|
  |                        |                          |   (consented + mandatory) |
  |                        |                          |<-- attribute list --------|
  |                        |                          |                           |
  |                        |                          |-- filter & union ---------|
  |                        |<-- filtered profile -----|                           |
  |<-- 200 filtered -------|                          |                           |
```
