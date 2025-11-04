## 📄 TESTING.md

### 🧪 Test Scenario: Comprehensive Profile Unification Logic Validation

This integration test verifies the **profile unification engine** of the Identity Customer Data Service (CDS).  
It simulates multiple real-world merging situations across temporary and permanent profiles, ensuring that all unification rules and priorities are respected and that profile hierarchies, attributes, and application data unify consistently.

---

### ✅ Pre-Test Setup

- All relevant **profile schema attributes** are registered before the test:
  - Identity attributes (email, phone number, user ID)
  - Trait attributes (interests)
  - Application data attributes (device IDs)
- **Unification rules** are configured with strict priorities:
  - `user_id` → `email` → `phone_number`
- The **profile service** and **unification service** are instantiated.
- Each test case simulates profile creation and updates that trigger unification.
- Asynchronous worker execution is stabilized using `time.Sleep()` delays.

---

### 🧬 Schema Attributes Added

| Attribute Name                    | Value Type       | Merge Strategy | Mutability |
|----------------------------------|------------------|----------------|-------------|
| `identity_attributes.email`      | string           | combine        | read_write  |
| `identity_attributes.phone_number` | string         | combine        | read_write  |
| `identity_attributes.user_id`    | string           | overwrite      | read_write  |
| `traits.interests`               | arrayOfString    | combine        | read_write  |
| `application_data.device_id`     | string           | combine        | read_write  |

---

### 🔗 Unification Rules Added

| Rule Name        | Property                        | Priority | Active |
|------------------|----------------------------------|----------|---------|
| user id based    | identity_attributes.user_id      | 0        | ✅ |
| email based      | identity_attributes.email        | 1        | ✅ |
| phone based      | identity_attributes.phone_number | 2        | ✅ |

---

### 👥 Test Scenarios

| #     | Scenario                                            | Description | Expected Outcome |
|-------|-----------------------------------------------------|-------------|------------------|
| **1** | Temp Profiles Cascading merge → Email → Phone Merge | Two temporary profiles share email, a third shares phone with one. | All three unify under one master; merged profile has both phone and email. |
| **2** | Temp + Permanent Merge                              | A temporary and permanent profile share the same email. | Unified profile inherits permanent’s `user_id`. |
| **3** | Two Temps → Then Permanent                          | Two temp profiles merge first by email, later unified with a permanent. | Final master includes `user_id` and all merged interests. |
| **4** | Temp + Perm → Another Temp                          | Existing unified profile merges with another temp via phone. | Master profile adds new phone + trait data. |
| **5** | Two Permanents, Same Email                          | Two permanent profiles share same email. | Should **not unify**; distinct master profiles remain. |
| **6** | Rule Priority                                       | Email has higher priority than phone for merge resolution. | Merge reason reflects `"email based"` rule. |
| **7** | Rule Inactivation                                   | Email rule temporarily disabled. | No merge occurs until rule is re-enabled. |
| **8** | Rule Change Stability                               | Existing merged profiles remain stable after rule changes. | Unification remains intact; hierarchy unchanged. |
| **9** | Cross-Tenant Isolation                              | Two profiles from different tenants share same identifiers. | Profiles remain separate; no cross-tenant unification. |
