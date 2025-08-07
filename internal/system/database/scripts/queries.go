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

var FilterProfileSchemaAttributes = map[string]string{
	"postgres": `SELECT attribute_id, tenant_id, attribute_name, value_type, merge_strategy, mutability, application_identifier, multi_valued, sub_attributes::text,
  canonical_values::text FROM profile_schema WHERE tenant_id = $1`,
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

var InsertProfile = map[string]string{
	"postgres": `
		INSERT INTO profiles (
		profile_id, user_id, tenant_id, created_at, updated_at, location, list_profile, delete_profile, traits, identity_attributes
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT (profile_id) DO NOTHING;`,
}

var InsertProfileReference = map[string]string{
	"postgres": `
		INSERT INTO profile_reference (profile_id, profile_status, reference_profile_id, reference_reason, tenant_id, reference_profile_tenant_id)
		VALUES ($1,$2,$3,$4, $5,$6)
		ON CONFLICT (profile_id) DO NOTHING;`,
}

var GetProfileById = map[string]string{
	"postgres": `
		SELECT p.profile_id, p.user_id, p.created_at, p.updated_at,p.location, p.tenant_id, p.list_profile, p.delete_profile, 
		       p.traits, p.identity_attributes, r.profile_status, r.reference_profile_id, r.reference_reason
		FROM 
			profiles p
		LEFT JOIN 
			profile_reference r ON p.profile_id = r.profile_id
		WHERE 
			p.profile_id = $1;`,
}

var GetAppDataByProfileId = map[string]string{
	"postgres": `SELECT app_id, application_data FROM application_data WHERE profile_id = $1;`,
}

var GetAppDataByAppId = map[string]string{
	"postgres": `SELECT app_id, application_data FROM application_data WHERE profile_id = $1 AND app_id = $2;`,
}

var UpdateProfile = map[string]string{
	"postgres": `
		UPDATE profiles SET
			user_id = $1,
			list_profile = $2,
			delete_profile = $3,
			traits = $4,
			identity_attributes = $5,
			updated_at = $6
		 WHERE profile_id = $7;`,
}

var UpsertProfileReference = map[string]string{
	"postgres": `
		UPDATE profile_reference SET
			profile_id = $1,
			profile_status = $2,
			reference_profile_id = $3,
			reference_reason = $4
		 WHERE profile_id = $5;`,
}
var UpdateProfileReference = map[string]string{
	"postgres": `
		UPDATE profile_reference
		SET reference_profile_id = $1,
			reference_reason = $2,
			profile_status = $3
		WHERE profile_id = $4`,
}

var GetProfilesByOrgId = map[string]string{
	"postgres": `
    SELECT 
        p.profile_id, 
        p.tenant_id, 
        p.created_at, 
        p.updated_at, 
        p.location, 
        p.user_id, 
        r.profile_status, 
        r.reference_profile_id, 
        r.reference_reason, 
        p.list_profile, 
        p.traits, 
        p.identity_attributes
    FROM 
        profiles p
    LEFT JOIN 
        profile_reference r ON p.profile_id = r.profile_id
    WHERE 
        p.list_profile = true 
        AND p.tenant_id = $1;`,
}

var DeleteProfileByProfileId = map[string]string{
	"postgres": `DELETE FROM application_data WHERE profile_id = $1`,
}

var InsertApplicationData = map[string]string{
	"postgres": `
		INSERT INTO application_data (profile_id, app_id, application_data)
		VALUES ($1, $2, $3)
		ON CONFLICT (profile_id, app_id)
		DO UPDATE SET application_data = EXCLUDED.application_data;
	`,
}

var DeleteProfileReference = map[string]string{
	"postgres": `DELETE FROM profile_reference WHERE reference_profile_id = $1 AND profile_id = $2;`,
}

var GetAllProfilesWithFilter = map[string]string{
	"postgres": `SELECT DISTINCT p.profile_id,
                p.user_id,
                p.tenant_id,
                p.created_at,
                p.updated_at,
                p.location,
                r.profile_status,
                r.reference_profile_id,
                r.reference_reason,
                p.list_profile,
                p.traits,
                p.identity_attributes
FROM profiles p
LEFT JOIN profile_reference r
    ON p.profile_id = r.profile_id`,
}

var GetAllReferenceProfileExceptCurrent = map[string]string{
	"postgres": `
	SELECT 
		p.profile_id, 
		p.user_id, 
		r.profile_status, 
		r.reference_profile_id, 
		r.reference_reason, 
		p.tenant_id,
		p.delete_profile,
		p.list_profile, 
		p.traits, 
		p.identity_attributes
	FROM 
		profiles p
	JOIN 
		profile_reference r ON p.profile_id = r.profile_id
	WHERE 
		r.profile_status = 'REFERENCE_PROFILE'
		AND p.profile_id != $1;`,
}

var FetchReferencedProfiles = map[string]string{
	"postgres": `
		SELECT profile_id, reference_reason, profile_status 
		FROM profile_reference 
		WHERE reference_profile_id = $1;`,
}

var GetProfileByUserId = map[string]string{
	"postgres": `
		SELECT p.profile_id, p.user_id, p.created_at, p.updated_at,p.location, p.tenant_id, p.list_profile, p.delete_profile, 
		       p.traits, p.identity_attributes, r.profile_status, r.reference_profile_id, r.reference_reason
		FROM 
			profiles p
		LEFT JOIN 
			profile_reference r ON p.profile_id = r.profile_id
		WHERE 
			p.user_id = $1
			AND r.profile_status = 'REFERENCE_PROFILE';`,
}

var InsertConsentCategory = map[string]string{
	"postgres": `INSERT INTO consent_categories (category_name, category_identifier, tenant_id, purpose, destinations)
				VALUES ($1, $2, $3, $4, $5)`,
}

var GetAllConsentCategories = map[string]string{
	"postgres": `SELECT category_name, category_identifier, tenant_id, purpose, destinations FROM consent_categories`,
}

var GetConsentCategoryById = map[string]string{
	"postgres": `SELECT category_name, category_identifier, tenant_id, purpose, destinations FROM consent_categories WHERE category_identifier = $1`,
}

var GetConsentCategoryByName = map[string]string{
	"postgres": `SELECT category_name, category_identifier, tenant_id, purpose, destinations FROM consent_categories WHERE category_name = $1`,
}

var UpdateConsentCategory = map[string]string{
	"postgres": `UPDATE consent_categories SET category_name=$1, purpose=$2, destinations=$3 WHERE category_identifier=$4`,
}

var DeleteConsentCategory = map[string]string{
	"postgres": `DELETE FROM consent_categories WHERE category_identifier=$1`,
}

var InsertCookie = map[string]string{
	"postgres": `INSERT INTO profile_cookies (cookie_id, profile_id, is_active) VALUES ($1, $2, $3)`,
}

var GetCookieByCookieId = map[string]string{
	"postgres": `SELECT cookie_id, profile_id, is_active FROM profile_cookies WHERE cookie_id = $1`,
}

var GetCookieByProfileId = map[string]string{
	"postgres": `SELECT cookie_id, profile_id, is_active FROM profile_cookies WHERE profile_id = $1`,
}

var UpdateCookieStatusByProfileId = map[string]string{
	"postgres": `UPDATE profile_cookies SET is_active = $1 WHERE profile_id = $2`,
}

var DeleteCookieById = map[string]string{
	"postgres": `DELETE FROM profile_cookies WHERE cookie_id = $1`,
}

var DeleteCookieByProfileId = map[string]string{
	"postgres": `DELETE FROM profile_cookies WHERE profile_id = $1`,
}
