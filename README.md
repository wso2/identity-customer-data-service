# identity-customer-data-service
Lightweight, extensible Customer Data Server built to power personalized experiences through unified user profiles and behavior insights.

# ⚡ Quickstart

### ✅ Prerequisites

- Go 1.23+
- Docker
- cURL

---
### 🛠️ Configuration Steps

1. Register an M2M Application
    - Go to WSO2 Identity Server Console.
    - Create a Machine-to-Machine application.
    - Under **Api Resources** section:
        - Subscribe to Claim Management APIs.
    - Under **Protocol** section
        - Copy the Client ID and Client Secret.

2. Create an Admin user
    - Go to WSO2 Identity Server Console .
    - Navigate to User Management section --> Users.
    - Create the user. Copy username and password
    - Navigate to Console Settings
        - Add administrator

3. Create the Environment File
    - Create a **dev.env** file under conf/repository/config/:
        - paste the Client secret copied
      ```bash
         ENV=dev
         LOG_LEVEL=DEBUG
         AUTH_SERVER_CLIENT_SECRET=<your_client_secret>
         AUTH_SERVER_ADMIN_PASSWORD=<your_admin_password>
      ```

4. Create Application that you want to use to access these APIs from
    - Register applications you want to integrate with the CDS.
    - Subscribe to required APIs from **API Resources** section:
        - Profiles
        - Profile Schema
        - Unification Rules
        - Consent Management

5. Obtain token against the application
    - Include required scopes for your use case.
    - required_scopes:
        - profile:create: "internal_cdm_profile_create"
        - profile:update: "internal_cdm_profile_update"
        - profile:view: "internal_cdm_profile_view"
        - profile:delete: "internal_cdm_profile_delete"

        - unification_rules:view: "internal_cdm_unification_rule_view"
        - unification_rules:create: "internal_cdm_unification_rule_create"
        - unification_rules:update: "internal_cdm_unification_rule_update"
        - unification_rules:delete: "internal_cdm_unification_rule_delete"

        - profile_schema:view: "internal_cdm_profile_schema_view"
        - profile_schema:create: "internal_cdm_profile_schema_create"
        - profile_schema:update: "internal_cdm_profile_schema_update"
        - profile_schema:delete: "internal_cdm_profile_schema_delete"

🎯 Include only the scopes you need in your token request.

```bash
`curl --location 'https://localhost:9443/oauth2/token' \
--header 'Authorization: Basic Wm1nNXJwVlRmcEhzN3RyYU01OWpwWG9tRFk4YTpaRTdJelJ3Sll6czlQVnJHd3JpZ3BwZ3g0VW42amVVV0plcXhZZDM1X0VjYQ==' \
--header 'Content-Type: application/x-www-form-urlencoded' \
--data-urlencode 'grant_type=client_credentials' \
--data-urlencode 'scope=internal_cdm_profile_update internal_cdm_profile_delete internal_cdm_profile_view internal_cdm_profile_create '`
```

6. Use the token obtained from token request call in Authorization header when calling CDM APIs.

`Authorization: Bearer <access_token>
`
---
## 🏗 Build and Run
### 🔧 Step 1: Start PostgreSQL

```bash
docker run -d -p 5432:5432 --name postgres \
  -e POSTGRES_USER=cdsuser \
  -e POSTGRES_PASSWORD=cdspwd \
  -e POSTGRES_DB=cdsdb \
  postgres
```

### 🗂 Step 2: Initialize the Database

```bash
docker exec -i postgres psql -U cdsuser -d cdspwd < dbscripts/postgress.sql
```

---

### 🛠 Step 3: Build the Product

```bash
make all
```

---

### ▶️ Step 4: Run the Product

```bash
cd target
unzip cds-1.0.0-m1-SNAPSHOT.zip
cd cds-1.0.0-m1-SNAPSHOT
./cds
```

---