## 📄 TESTING.md

### 🧪 Test Scenario: End-to-End Profile Enrichment and Unification

This integration test validates the **enrichment** and **unification** workflows in the identity customer data service. It simulates the generation of events across four distinct profiles using different devices and applications and ensures that all enrichment rules are applied and the profiles are unified into a single master profile based on configured rules.

---

### ✅ Pre-Test Setup

- The profile worker is started using `StartProfileWorker()`.
- Enrichment and unification rules are added before any event processing.
- Test profiles, devices, and applications are uniquely identified.
- Events are injected with slight delays to allow sequential processing.

---

### 🧬 Enrichment Rules Added

| Property                            | Source Field   | Event Type | Event Name             | Merge Strategy | Conditions                          |
|-------------------------------------|----------------|------------|------------------------|----------------|-------------------------------------|
| `identity_attributes.email`         | `email`        | identify   | user_logged_in         | combine        | –                                   |
| `identity_attributes.email`         | `email`        | track      | newsletter_subscribed  | combine        | `newsletter_subscribed = true`     |
| `identity_attributes.phone_number`  | `mobile_number`| track      | purchase_initiated     | combine        | –                                   |
| `identity_attributes.phone_number`  | `mobile_number`| identify   | user_logged_in         | combine        | –                                   |
| `identity_attributes.user_name`     | `user_name`    | identify   | user_logged_in         | overwrite      | –                                   |
| `identity_attributes.user_id`       | `user_id`      | identify   | user_logged_in         | overwrite      | –                                   |
| `traits.interests`                  | `objectname`   | track      | category_searched      | combine        | `action = select_category`         |

---

### 🔗 Unification Rules Added

| Rule Name        | Property                        | Priority |
|------------------|----------------------------------|----------|
| user id based    | identity_attributes.user_id      | 1        |
| email based      | identity_attributes.email        | 2        |
| phone based      | identity_attributes.phone_number | 3        |

---

### 👥 Profile Simulation

| Profile     | Device     | App       | Events                                                                                       |
|-------------|------------|-----------|----------------------------------------------------------------------------------------------|
| profile_1   | device_1   | app_1     | page_visited, guest_user_session, category_searched (A), newsletter_subscribed, user_logged_in |
| profile_2   | device_2   | app_1     | page_visited, purchase_initiated (07x1234567), category_searched (B)                        |
| profile_3   | device_3   | app_1     | page_visited, category_searched (C), purchase_initiated (07x9876543), newsletter_subscribed |
| profile_4   | device_4   | app_2     | page_visited, category_searched (D), user_logged_in                                         |

---

### 🔄 Expected Profile Unification Chain

- **Step 1**: `profile_1` unified with `profile_2` via `phone_number`
- **Step 2**: `profile_1` unified with `profile_3` via `email`
- **Step 3**: `profile_1` unified with `profile_4` via `user_id`


Result: One master profile with all four profiles as children.

---

### 📊 Final Assertions

#### 📌 Master Profile
- **Emails**: `email1@gmail.com`, `email2@gmail.com`
- **Phone Numbers**: `07x1234567`, `07x9876543`
- **User ID**: `user-1-id`
- **User Name**: `user1`
- **Interests**: `A`, `B`, `C`, `D`
- **Applications**: `app_1`, `app_2`
- **Devices**: `device_1`, `device_2`, `device_3`, `device_4`

#### 🧩 Child Profiles Validation
- All 4 profiles point to the same `parent_profile_id`
- Child profiles and their respective unification rules:
  - `profile_1`: `phone based`
  - `profile_2`: `phone based`
  - `profile_3`: `email based`
  - `profile_4`: `user id based`

#### 📦 Application Data
- Master profile includes 2 `application_data` entries
- Each entry contains merged `devices`
- All 4 unique `device_id`s are present

---

### 📝 Notes
- The merging logic for devices works even without explicit `application_data` enrichment rules.
- Application and device merging occurs during profile unification.
- Event ordering is mostly preserved, but workers operate concurrently. Ensuring slight delays improves reliability.

---

### ✅ Test Outcome
This test verifies the system's ability to:
- Enrich profile attributes based on dynamic rules
- Merge profiles hierarchically according to defined unification logic
- Preserve device data during unification
- Maintain traceability through child profile mapping and rule tracking
