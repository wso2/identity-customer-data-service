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
      ├── enrichment_rules_test.go
      ├── events_test.go
      ├── profile_test.go
      ├── unification_rules_test.go
      ├── event_stream_id_test.go
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

| Test Suite             | Sub-Test Name             | Description                                       | Status  |
|------------------------|---------------------------|---------------------------------------------------|---------|
| `Test_EnrichmentRule`  | Add_enrichment_rule       | Adds profile enrichment rules                     | ✅ Pass |
|                        | Update_enrichment_rule    | Updates an enrichment rule                        | ✅ Pass |
|                        | Delete_enrichment_rule    | Deletes the enrichment rule                       | ✅ Pass |
| `Test_EventStreamId`   | Create_event_stream_Id    | Creates a new event stream ID                     | ✅ Pass |
|                        | Fetch_event_stream_Id_by_app | Retrieves by app/org                              | ✅ Pass |
|                        | Get_event_stream_Id_by_Id | Fetches by ID                                     | ✅ Pass |
|                        | Rotate_event_stream_Id    | Rotates the stream ID                             | ✅ Pass |
|                        | Revoke_event_stream_Id    | Revokes the stream ID                             | ✅ Pass |
| `Test_Events`          | Add_event                 | Ingests an event                                  | ✅ Pass |
|                        | Get_Profile               | Verifies profile linkage                          | ✅ Pass |
|                        | Get_event_by_ID           | Fetches event by ID                               | ✅ Pass |
|                        | Get_events_with_filters   | Filters events                                    | ✅ Pass |
| `Test_Profiles`        | Create_profile_via_event  | Creates profile from event ingestion              | ✅ Pass |
|                        | Fetch_created_profile     | Retrieves created profile                         | ✅ Pass |
|                        | Get_all_profiles          | Lists all profiles                                | ✅ Pass |
|                        | Filter_profiles           | Filters by enriched email                         | ✅ Pass |
|                        | Delete_profile            | Deletes profile and hierarchy                     | ✅ Pass |
| `Test_UnificationRule` | Pre-requisite: Add_enrichment_rule | Sets up enrichment rule for validation            | ✅ Pass |
|                        | Add_unification_rule      | Adds a unification rule                           | ✅ Pass |
|                        | Get_all_unification_rules | Lists rules                                       | ✅ Pass |
|                        | Update_unification_rule   | Updates a rule                                    | ✅ Pass |
|                        | Delete_unification_rule   | Removes rule                                      | ✅ Pass |

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
