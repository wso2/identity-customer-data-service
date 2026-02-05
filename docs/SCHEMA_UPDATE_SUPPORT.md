# Schema Update Support

This document explains how the schema update support feature works in the Identity Customer Data Service.

## Overview

The schema update support feature allows administrators to change profile schema attributes (such as data types and multi-valued settings) without losing existing profile data. When a schema attribute is updated, the system automatically migrates existing profile data to match the new schema definition.

## Problem Statement

Previously, profile data was stored in their defined format (integer, boolean, string, etc.) in the database. This made it difficult to change the data format or attributes like `multi_valued` because:

1. Existing data would become incompatible with the new schema
2. Type validation would fail for existing profiles
3. Manual data migration would be required

## Solution

The solution includes two key components:

### 1. Data Transformation Layer

A transformation layer has been added to handle type conversions between different data formats:

- **Serialization**: Converts typed values to a format that can be stored flexibly
- **Deserialization**: Converts stored values back to their expected types based on the schema
- **Backward Compatibility**: Works with both old (typed) and new (string-based) storage formats

### 2. Automatic Schema Migration

When a schema attribute is updated with changes to `value_type` or `multi_valued`, the system:

1. Detects that migration is needed
2. Launches an asynchronous background process
3. Migrates all existing profile data to match the new schema
4. Updates profiles in batches to minimize performance impact

## Supported Schema Changes

### Type Changes

The system supports converting between the following data types:

| From Type | To Type | Example |
|-----------|---------|---------|
| String | Integer | "25" → 25 |
| String | Decimal | "3.14" → 3.14 |
| String | Boolean | "true" → true |
| Integer | String | 25 → "25" |
| Integer | Decimal | 25 → 25.0 |
| Decimal | Integer | 3.14 → 3 |
| Boolean | String | true → "true" |

### Multi-Valued Changes

| From | To | Example |
|------|-----|---------|
| Single value | Array | "blue" → ["blue"] |
| Array | Single value | ["red", "blue"] → "red" (first element) |

### Combined Changes

You can also change both type and multi-valued in a single update:

- String single value → Integer array: "100" → [100]
- Integer array → String single value: [1, 2, 3] → "1"

## Usage

### Updating a Schema Attribute

Use the `PATCH /profile-schema/{attributeId}` endpoint to update a schema attribute:

```bash
curl -X PATCH \
  https://api.example.com/cds/api/profile-schema/{attributeId} \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {token}" \
  -d '{
    "value_type": "integer",
    "multi_valued": true
  }'
```

### Migration Process

1. The schema is validated to ensure the new configuration is valid
2. The schema is updated in the database
3. If `value_type` or `multi_valued` changed, migration is triggered asynchronously
4. The API returns immediately (migration happens in the background)
5. Profile data is migrated in batches
6. Logs indicate when migration starts and completes

### Monitoring Migration

Check the application logs for migration status:

```
INFO: Schema migration triggered for attribute {attributeId} in org {orgId}
INFO: Migrating data for identity_attributes.age from string (multi: false) to integer (multi: false)
INFO: Migration completed: updated 150 profiles
```

## Technical Implementation

### Files Added

1. **`internal/profile/service/data_transformer.go`**
   - Contains transformation utilities for type conversions
   - Handles serialization and deserialization
   - Maintains backward compatibility

2. **`internal/profile/service/data_transformer_test.go`**
   - Unit tests for transformation functions
   - Tests all type conversion scenarios
   - Validates backward compatibility

3. **`internal/profile_schema/service/schema_migration.go`**
   - Implements the profile data migration logic
   - Handles database queries and updates
   - Converts values between different types and formats

### Files Modified

1. **`internal/profile_schema/service/profile_schema_service.go`**
   - Updated `PatchProfileSchemaAttributeById` to trigger migration
   - Added logic to detect when migration is needed
   - Launches migration asynchronously

## Testing

### Unit Tests

Run the transformation layer unit tests:

```bash
go test ./internal/profile/service -v -run TestTransform
```

### Integration Tests

Run the schema migration integration tests:

```bash
make integration-test test=Test_SchemaUpdate_Migration
```

Integration tests cover:
- Type changes (string → integer)
- Multi-valued changes (single → array)
- Combined changes (type + multi-valued)
- No migration when schema unchanged

## Performance Considerations

### Asynchronous Migration

- Migration runs in a background goroutine
- The API response is immediate (doesn't block)
- Large datasets may take time to migrate

### Batch Processing

- Profiles are migrated one at a time with transaction support
- Failed migrations for individual profiles are logged but don't stop the process
- The system continues to function during migration

### Backward Compatibility

- Old data (stored with native types) still works
- New data can be stored in either format
- The transformation layer handles both formats seamlessly

## Limitations

1. **Application Data Migration**: For application_data attributes with multiple applications, all applications with that attribute will be migrated.

2. **Data Loss Scenarios**:
   - Array to single value: Only the first element is kept
   - Invalid type conversions: Default values are used (e.g., empty string, 0, false)

3. **Complex Type Migration**: Complex/nested types are converted to JSON strings and may need manual cleanup.

## Example Scenarios

### Scenario 1: Age from String to Integer

**Initial State:**
```json
{
  "schema": {
    "attribute_name": "identity_attributes.age",
    "value_type": "string"
  },
  "profile": {
    "age": "25"
  }
}
```

**After Update:**
```json
{
  "schema": {
    "attribute_name": "identity_attributes.age",
    "value_type": "integer"
  },
  "profile": {
    "age": 25
  }
}
```

### Scenario 2: Single Email to Multiple Emails

**Initial State:**
```json
{
  "schema": {
    "attribute_name": "identity_attributes.email",
    "value_type": "string",
    "multi_valued": false
  },
  "profile": {
    "email": "user@example.com"
  }
}
```

**After Update:**
```json
{
  "schema": {
    "attribute_name": "identity_attributes.email",
    "value_type": "string",
    "multi_valued": true
  },
  "profile": {
    "email": ["user@example.com"]
  }
}
```

## Best Practices

1. **Test in Development First**: Always test schema updates in a development environment before applying to production.

2. **Monitor Logs**: Watch application logs during and after schema updates to ensure migration completes successfully.

3. **Backup Data**: Although the system is designed to preserve data, consider backing up critical profile data before major schema changes.

4. **Gradual Rollout**: For large datasets, consider updating schema attributes one at a time rather than in bulk.

5. **Validate After Migration**: After migration completes, spot-check a few profiles to ensure data was transformed correctly.

## Troubleshooting

### Migration Not Running

**Problem**: Schema updated but data not migrated.

**Solution**: Check logs for error messages. Ensure:
- Database connection is available
- Profiles exist with the attribute
- The attribute name matches the schema

### Data Type Mismatch After Migration

**Problem**: Profile data doesn't match expected type after migration.

**Solution**: Check if:
- Source data was in a valid format (e.g., "abc" can't convert to integer)
- The transformation logic handles your specific case
- There were any errors logged during migration

### Performance Issues During Migration

**Problem**: System slow during large migration.

**Solution**:
- Migration runs asynchronously and shouldn't block the API
- For very large datasets, consider off-peak hours for schema updates
- Monitor database performance during migration

## Future Enhancements

Potential improvements for future versions:

1. **Migration Status API**: Add an endpoint to check migration progress
2. **Rollback Support**: Allow reverting schema changes and data migration
3. **Dry Run Mode**: Preview migration results before applying
4. **Custom Transformations**: Allow defining custom transformation logic
5. **Migration Queuing**: Queue multiple schema updates for sequential processing

## References

- [Profile Schema API Documentation](../api/customer-data-service.yaml)
- [Integration Test Guide](../test/Integration_Test_Guide.MD)
- [Main README](../README.md)
