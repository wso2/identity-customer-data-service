---
title: Integration Testing Guide
date: 2025-05-21
---

# 🧪 Testing Guide — Identity Customer Data Service

This guide explains how to run, manage, and report integration tests in the `identity-customer-data-service` project using Go, Testcontainers, and standard tooling.

---

##  Prerequisites

1. **Install Go** (v1.20+ recommended)  
2. **Install Docker** (with CLI access)  

---

##  Test Directory Structure

```
/test
  └── integration
      ├── profile_schema_test.go
      ├── profile_test.go
      ├── unification_rules_test.go
      ├── setup/
      │    └── schema.sql
      └── utils/
           └── ...
```

---

##  Test Setup Overview

Tests use **Testcontainers-Go** to spin up a Postgres container and initialize the schema from `test/setup/schema.sql`.

- Containers are terminated explicitly at the end.
- `TESTCONTAINERS_RYUK_DISABLED=true` is used to avoid lifecycle issues.

---

## 🧪 Running Tests

### Run All Tests

```bash
make integration-test
```

Logs are stored in:

```
target/integration-test.log
```

### Run a Specific Test

```bash
make integration-test test=Test_ProfilesService
```

---

## ✅ Test Suite Summary

| Test Suite             | Sub-Test Name             | Description                            | Status  |
|------------------------|---------------------------|----------------------------------------|---------|
| `Test_Profiles`        | Create_profile_via_event  | Creates profile from event ingestion   | ✅ Pass |
|                        | Fetch_created_profile     | Retrieves created profile              | ✅ Pass |
|                        | Get_all_profiles          | Lists all profiles                     | ✅ Pass |
|                        | Filter_profiles           | Filters by enriched email              | ✅ Pass |
|                        | Delete_profile            | Deletes profile and hierarchy          | ✅ Pass |
| `Test_UnificationRule` | Pre-requisite: Add_schema_attribute | Sets up enrichment rule for validation | ✅ Pass |
|                        | Add_unification_rule      | Adds a unification rule                | ✅ Pass |
|                        | Get_all_unification_rules | Lists rules                            | ✅ Pass |
|                        | Update_unification_rule   | Updates a rule                         | ✅ Pass |
|                        | Delete_unification_rule   | Removes rule                           | ✅ Pass |
| `Test_Consent` | Add_consent_category | Add consent category                   | ✅ Pass |
|                        | Get_all_consent_categories      | List consent categories                | ✅ Pass |
|                        | Get_single_category | Fetch single consent category          | ✅ Pass |
|                        | Update_consent_category   | Updates a consent category             | ✅ Pass |
|                        | Delete_consent_category   | Removes consent category                           
---

## 🛠️ Tooling Reference

| Tool                | Purpose                                  |
|---------------------|-------------------------------------------|
| `gotestsum`         | Structured test runner with rich output   |
| `testcontainers-go` | Spin up ephemeral Postgres test DB        |
| `stretchr/testify`  | Assertions and suite support              |

---

## 🧹 Cleanup

To clean all build/test artifacts:

```bash
make clean
```

---

## ⚠️ Notes

- Docker **must be running**.
- Tests modify DB state — run in isolation.
- Profile enrichment and unification happen **asynchronously** via queue workers — some tests include `sleep` or retry logic.

---
