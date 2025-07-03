/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package scripts

var DeleteProfileSchemaForOrg = map[string]string{
	"postgres": `
        DELETE FROM profile_schema WHERE tenant_id = $1 AND scope != 'identity_attributes' `,
}

var GetProfileSchemaByOrg = map[string]string{
	"postgres": `SELECT attribute_id, attribute_name, value_type, merge_strategy , application_identifier, mutability, 
       multi_valued, sub_attributes::text, canonical_values::text FROM profile_schema WHERE tenant_id = $1`,
}

var DeleteIdentityClaimsOfProfileSchema = map[string]string{
	"postgres": `DELETE FROM profile_schema WHERE tenant_id = $1 AND scope = 'identity_attributes'`,
}

var InsertIdentityClaimsForProfileSchema = map[string]string{
	"postgres": `INSERT INTO profile_schema 
	(tenant_id, attribute_id, attribute_name, value_type, merge_strategy, mutability, application_identifier, 
	 multi_valued, canonical_values, sub_attributes, scim_dialect, scope) VALUES `,
}

var GetProfileSchemaAttributeByName = map[string]string{
	"postgres": `SELECT attribute_id, attribute_name, value_type, merge_strategy, mutability , application_identifier, 
       multi_valued, sub_attributes::text, canonical_values::text FROM profile_schema WHERE tenant_id = $1 
       AND attribute_name = $2 LIMIT 1`,
}

var InsertProfileSchemaAttributesForScope = map[string]string{
	"postgres": `INSERT INTO profile_schema (tenant_id, attribute_id, attribute_name, value_type, merge_strategy, 
                            application_identifier, mutability, multi_valued, sub_attributes, canonical_values, scope) VALUES `,
}
var GetProfileSchemaAttributeByScope = map[string]string{
	"postgres": `SELECT attribute_id, tenant_id, attribute_name, value_type, merge_strategy, mutability, application_identifier, multi_valued,   sub_attributes::text,
  canonical_values::text FROM profile_schema WHERE tenant_id = $1 AND scope = $2`,
}

var UpdateProfileSchemaAttributesForSchema = map[string]string{
	"postgres": `
		UPDATE profile_schema
		SET attribute_name = $1,
			value_type = $2,
			merge_strategy = $3,
			mutability = $4,
			application_identifier = $5,
			multi_valued = $6,
			canonical_values = $7,
			sub_attributes = $8
		WHERE tenant_id = $9 AND attribute_id = $10 AND scope = $11
	`,
}

var DeleteProfileSchemaAttributeForScope = map[string]string{
	"postgres": `DELETE FROM profile_schema WHERE tenant_id = $1 AND scope =  $2`,
}

var GetProfileSchemaAttributeById = map[string]string{
	"postgres": `SELECT attribute_id, attribute_name, value_type, merge_strategy, mutability , application_identifier, multi_valued,   sub_attributes::text,
  canonical_values::text
	          FROM profile_schema WHERE tenant_id = $1 AND attribute_id = $2`,
}

var DeleteProfileSchemaAttributeById = map[string]string{
	"postgres": `DELETE FROM profile_schema WHERE tenant_id = $1 AND attribute_id = $2`,
}

var GetUnificationRules = map[string]string{
	"postgres": `SELECT rule_id, rule_name, property_name, priority, is_active, created_at, updated_at 
FROM unification_rules WHERE tenant_id = $1`,
}

var GetUnificationRule = map[string]string{
	"postgres": `SELECT rule_id, rule_name, property_name, priority, is_active, created_at, updated_at FROM unification_rules WHERE rule_id = $1`,
}

var DeleteUnificationRule = map[string]string{
	"postgres": `DELETE FROM unification_rules WHERE rule_id = $1`,
}
var InsertUnificationRule = map[string]string{
	"postgres": `INSERT INTO unification_rules (rule_id, tenant_id, rule_name, property_name, priority, is_active, created_at, updated_at) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
}
