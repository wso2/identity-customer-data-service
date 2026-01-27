# Customer Data Service API Documentation

This directory contains the OpenAPI (Swagger) specification for the Customer Data Service API.

## Files

- `customer-data-service.yaml` - Complete OpenAPI 3.0 specification

## What's Included

The OpenAPI specification provides comprehensive documentation for:

### Endpoints
- **Profile Management** - Create, read, update, delete customer profiles
- **Profile Schema** - Define and manage extensible profile attributes
- **Event Tracking** - Record and query customer behavior events
- **Profile Unification** - Configure rules for merging profiles
- **Profile Enrichment** - Set up computed profile attributes
- **Consent Management** - Handle consent preferences and categories
- **Health & Readiness** - Service health monitoring endpoints

### Authentication
- **Bearer JWT** - Token-based authentication with scope permissions
- **Basic Auth** - Username/password authentication for admin operations

### Features
- Complete request/response schemas
- Error response definitions
- Query parameters for filtering and pagination
- Multi-tenant support with tenant path variables
- Comprehensive field descriptions and examples

## Using the Specification

### 1. View Online

Upload `customer-data-service.yaml` to:
- [Swagger Editor](https://editor.swagger.io/) - Interactive editor and preview
- [Redoc](https://redocly.github.io/redoc/) - Beautiful documentation viewer

### 2. Generate Client SDKs

Use [OpenAPI Generator](https://openapi-generator.tech/) to generate client libraries:

```bash
# JavaScript/TypeScript
openapi-generator-cli generate -i customer-data-service.yaml -g typescript-axios -o ./client

# Python
openapi-generator-cli generate -i customer-data-service.yaml -g python -o ./client

# Java
openapi-generator-cli generate -i customer-data-service.yaml -g java -o ./client

# Go
openapi-generator-cli generate -i customer-data-service.yaml -g go -o ./client
```

### 3. Serve Documentation Locally

#### Using Redoc
```bash
npx @redocly/cli preview-docs customer-data-service.yaml
```

#### Using Swagger UI
```bash
docker run -p 8080:8080 -e SWAGGER_JSON=/api/customer-data-service.yaml \
  -v $(pwd):/api swaggerapi/swagger-ui
```

Then open http://localhost:8080

### 4. Validate the Specification

```bash
# Install validator
npm install -g @apidevtools/swagger-cli

# Validate
swagger-cli validate customer-data-service.yaml
```

### 5. API Testing

Import the specification into:
- [Postman](https://www.postman.com/) - Import OpenAPI spec for automated collection generation
- [Insomnia](https://insomnia.rest/) - Generate requests from OpenAPI
- [HTTPie](https://httpie.io/) - Command-line HTTP client with OpenAPI support

## Multi-tenancy

All endpoints (except health checks) require a tenant identifier in the URL path:

```
/t/{tenant}/cds/api/v1/profiles
```

Default tenant: `carbon.super`

## Authentication Example

### Get JWT Token
```bash
curl --location 'https://localhost:9443/oauth2/token' \
  --header 'Authorization: Basic <Base64Encoded(CLIENT_ID:CLIENT_SECRET)>' \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'grant_type=client_credentials' \
  --data-urlencode 'scope=internal_cdm_profile_view internal_cdm_profile_create'
```

### Use Token
```bash
curl --location 'http://localhost:8080/t/carbon.super/cds/api/v1/profiles' \
  --header 'Authorization: Bearer <access_token>'
```

## Required Scopes

The specification documents the following scope-based permissions:

### Profile Management
- `profile:view` - View profiles
- `profile:create` - Create profiles
- `profile:update` - Update profiles
- `profile:delete` - Delete profiles

### Profile Schema
- `profile_schema:view` - View schema
- `profile_schema:create` - Create schema attributes
- `profile_schema:update` - Update schema attributes
- `profile_schema:delete` - Delete schema attributes

### Unification Rules
- `unification_rules:view` - View rules
- `unification_rules:create` - Create rules
- `unification_rules:update` - Update rules
- `unification_rules:delete` - Delete rules

### Consent Management
- `consent_category:view` - View consent categories
- `consent_category:create` - Create categories
- `consent_category:update` - Update categories
- `consent_category:delete` - Delete categories

## Error Handling

All errors follow a standard format:

```json
{
  "code": "ERROR_CODE",
  "message": "Human-readable message",
  "description": "Detailed description",
  "trace_id": "request-trace-id"
}
```

Common HTTP status codes:
- `200` - Success
- `201` - Created
- `204` - No Content (successful deletion)
- `400` - Bad Request
- `401` - Unauthorized
- `403` - Forbidden
- `404` - Not Found
- `500` - Internal Server Error

## Contributing

When updating the API specification:

1. Make changes to `customer-data-service.yaml`
2. Validate the specification:
   ```bash
   swagger-cli validate customer-data-service.yaml
   ```
3. Test with actual API implementation
4. Update this README if adding major features
5. Commit changes with descriptive messages

## Support

For questions or issues:
- Check the main [README](../README.md) for general project information
- Review the OpenAPI specification for detailed endpoint documentation
- Contact the WSO2 Identity team

## License

Apache 2.0 - See [LICENSE](../LICENSE) for details
