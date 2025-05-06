# identity-customer-data-service
Lightweight, extensible Customer Data Server built to power personalized experiences through unified user profiles and behavior insights.

## ⚡ Quickstart

### ✅ Prerequisites

- Go 1.23+
- Docker
- cURL

---

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