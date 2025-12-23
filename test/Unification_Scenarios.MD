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
| **10** | Multiple Cascading Merges                          | Four profiles chain together - P1, P2 by email, P2, P3 by phone, P3, P4 by email. | All four merge into one master with combined data. |
| **11** | Deep Hierarchy Unification                         | Two profiles merge first, then a third joins via phone match. | All three end up in same hierarchy with all interests combined. |
| **12** | Application Data Merge                             | Profiles with different application data merge together. | App data from both profiles gets combined properly. |
| **13** | Update Triggers Reunification                      | Two separate profiles exist, one gets updated to add matching email. | After update, both profiles merge together. |
| **14** | Multiple Attributes Match                          | Two profiles match on both email AND phone at the same time. | They merge once using higher priority rule (email). |
| **15** | Four Profile Chain                                 | Chain merge A to B to C to D through email and phone links. | All four merge with all preferences combined. |
| **16** | Permanent with Multiple Temps                      | One permanent profile, three temps join via email and phone. | All temps merge under the permanent profile. |
| **17** | Different Trait Types Merge                        | Profiles have arrays (interests) and scalars (score). | Arrays combine, scalars use last value. |
| **18** | Rule Reactivation                                  | Rule disabled, profiles created, rule re-enabled, new profile added. | New profile triggers merge after rule reactivation. |
| **19** | Two Hierarchies Merge                              | Two separate master hierarchies (3 children + 2 children) connected by new profile. | All profiles end up in single unified hierarchy. |
