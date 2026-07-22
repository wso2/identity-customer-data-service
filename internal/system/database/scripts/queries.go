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

// UpsertApplication inserts or updates the application information.
var UpsertApplication = map[string]string{
	"postgres": `INSERT INTO applications (app_id, org_handle, client_id, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (app_id) DO UPDATE SET
			org_handle = EXCLUDED.org_handle,
			client_id  = EXCLUDED.client_id,
			updated_at = now()`,
}

// GetAppIdentifierByClientID resolves an OAuth clientId to the app_id.
var GetAppIdentifierByClientID = map[string]string{
	"postgres": `SELECT app_id FROM applications
		WHERE org_handle = $1 AND client_id = $2 LIMIT 1`,
}

var DeleteProfileSchemaForOrg = map[string]string{
	"postgres": `
        DELETE FROM profile_schema WHERE org_handle = $1 AND scope != 'identity_attributes' `,
}

var GetProfileSchemaByOrg = map[string]string{
	"postgres": `SELECT attribute_id, attribute_name, display_name, value_type, merge_strategy , application_identifier, mutability, 
       multi_valued, sub_attributes::text, canonical_values::text FROM profile_schema WHERE org_handle = $1`,
}

var DeleteIdentityClaimsOfProfileSchema = map[string]string{
	"postgres": `DELETE FROM profile_schema WHERE org_handle = $1 AND scope = 'identity_attributes'`,
}

var InsertIdentityClaimsForProfileSchema = map[string]string{
	"postgres": `INSERT INTO profile_schema
	(org_handle, attribute_id, attribute_name, value_type, merge_strategy, mutability, application_identifier,
	 multi_valued, canonical_values, sub_attributes, scim_dialect, scope, display_name) VALUES `,
}

// UpsertIdentityClaimsForProfileSchema inserts or updates identity attributes in place,
// preserving the attribute_id so that FK references (e.g. unification_rules) are not broken.
var UpsertIdentityClaimsForProfileSchema = map[string]string{
	"postgres": `INSERT INTO profile_schema
	(org_handle, attribute_id, attribute_name, value_type, merge_strategy, mutability, application_identifier,
	 multi_valued, canonical_values, sub_attributes, scim_dialect, scope, display_name) VALUES
	%s
	ON CONFLICT (attribute_id) DO UPDATE SET
		attribute_name         = EXCLUDED.attribute_name,
		value_type             = EXCLUDED.value_type,
		merge_strategy         = EXCLUDED.merge_strategy,
		mutability             = EXCLUDED.mutability,
		application_identifier = EXCLUDED.application_identifier,
		multi_valued           = EXCLUDED.multi_valued,
		canonical_values       = EXCLUDED.canonical_values,
		sub_attributes         = EXCLUDED.sub_attributes,
		scim_dialect           = EXCLUDED.scim_dialect,
		display_name           = EXCLUDED.display_name`,
}

var GetProfileSchemaAttributeByName = map[string]string{
	"postgres": `SELECT attribute_id, attribute_name, display_name, value_type, merge_strategy, mutability, application_identifier,
       multi_valued, sub_attributes::text, canonical_values::text, scope FROM profile_schema WHERE org_handle = $1
       AND attribute_name = $2 LIMIT 1`,
}

var InsertProfileSchemaAttributesForScope = map[string]string{
	"postgres": `INSERT INTO profile_schema (org_handle, attribute_id, attribute_name, value_type, merge_strategy,
                            application_identifier, mutability, multi_valued, sub_attributes, canonical_values, scope, display_name) VALUES `,
}
var GetProfileSchemaAttributeByScope = map[string]string{
	"postgres": `SELECT attribute_id, org_handle, attribute_name, display_name, value_type, merge_strategy, mutability, application_identifier, multi_valued,   sub_attributes::text,
  canonical_values::text FROM profile_schema WHERE org_handle = $1 AND scope = $2`,
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
			sub_attributes = $8,
			display_name = $9
		WHERE org_handle = $10 AND attribute_id = $11 AND scope = $12
	`,
}

var DeleteProfileSchemaAttributeForScope = map[string]string{
	"postgres": `DELETE FROM profile_schema WHERE org_handle = $1 AND scope =  $2`,
}

var GetProfileSchemaAttributeById = map[string]string{
	"postgres": `SELECT attribute_id, attribute_name, display_name, value_type, merge_strategy, mutability, application_identifier, multi_valued, sub_attributes::text,
  canonical_values::text, scope
	          FROM profile_schema WHERE org_handle = $1 AND attribute_id = $2`,
}

var FilterProfileSchemaAttributes = map[string]string{
	"postgres": `SELECT attribute_id, org_handle, attribute_name, display_name, value_type, merge_strategy, mutability, application_identifier, multi_valued, sub_attributes::text,
  canonical_values::text FROM profile_schema WHERE org_handle = $1`,
}

var DeleteProfileSchemaAttributeById = map[string]string{
	"postgres": `DELETE FROM profile_schema WHERE org_handle = $1 AND attribute_id = $2`,
}

var GetUnificationRules = map[string]string{
	"postgres": `SELECT rule_id, rule_name, property_name, property_id, priority, is_active, attribute_type, unification_method, created_at, updated_at
FROM unification_rules WHERE org_handle = $1`,
}

var GetUnificationRule = map[string]string{
	"postgres": `SELECT rule_id, rule_name, property_name, property_id, priority, is_active, attribute_type, unification_method, created_at, updated_at FROM unification_rules WHERE rule_id = $1`,
}

var DeleteUnificationRule = map[string]string{
	"postgres": `DELETE FROM unification_rules WHERE rule_id = $1`,
}
var InsertUnificationRule = map[string]string{
	"postgres": `INSERT INTO unification_rules (rule_id, org_handle, rule_name, property_name, property_id, priority, is_active, attribute_type, unification_method, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
}

var UpdateUnificationRule = map[string]string{
	"postgres": `UPDATE unification_rules SET rule_name = $1, priority = $2, is_active = $3, attribute_type = $4, unification_method = $5, updated_at = $6
		 WHERE rule_id = $7;`,
}

var InsertProfile = map[string]string{
	"postgres": `
		INSERT INTO profiles (
		profile_id, user_id, org_handle, created_at, updated_at, location, list_profile, delete_profile, traits, identity_attributes
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT (profile_id) DO NOTHING;`,
}

var InsertProfileReference = map[string]string{
	"postgres": `
		INSERT INTO profile_reference (profile_id, profile_status, reference_profile_id, reference_reason, org_handle, reference_profile_org_handle)
		VALUES ($1,$2,$3,$4, $5,$6)
		ON CONFLICT (profile_id) DO NOTHING;`,
}

var GetProfileById = map[string]string{
	"postgres": `
		SELECT p.profile_id, p.user_id, p.created_at, p.updated_at,p.location, p.org_handle, p.list_profile, p.delete_profile, 
		       p.traits, p.identity_attributes, r.profile_status, r.reference_profile_id, r.reference_reason
		FROM 
			profiles p
		LEFT JOIN 
			profile_reference r ON p.profile_id = r.profile_id
		WHERE 
			p.profile_id = $1;`,
}

var GetProfileConsentsByProfileId = map[string]string{
	"postgres": `SELECT profile_id, category_id, consent_status, consented_at FROM profile_consents WHERE profile_id = $1;`,
}

var DeleteProfileConsentsByProfileId = map[string]string{
	"postgres": `DELETE FROM profile_consents WHERE profile_id = $1;`,
}

var InsertProfileConsentsByProfileId = map[string]string{
	"postgres": `INSERT INTO profile_consents (profile_id, category_id, consent_status, consented_at) VALUES ($1, $2, $3, $4)`,
}

var GetAppDataByProfileId = map[string]string{
	"postgres": `SELECT app_id, application_data FROM application_data WHERE profile_id = $1;`,
}

var GetAppDataByProfileIds = map[string]string{
	"postgres": `SELECT profile_id, app_id, application_data FROM application_data WHERE profile_id IN (%s);`,
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
			p.org_handle, 
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
		FROM profiles p
		LEFT JOIN profile_reference r ON p.profile_id = r.profile_id
		WHERE 
			r.profile_status = 'REFERENCE_PROFILE'
			AND p.org_handle = $1
			AND (
				$2::timestamptz IS NULL
				OR (
					($4 = 'next' AND (p.created_at, p.profile_id) < ($2::timestamptz, $3::text))
					OR
					($4 = 'prev' AND (p.created_at, p.profile_id) > ($2::timestamptz, $3::text))
				)
			)
		ORDER BY 
			CASE WHEN $4 = 'prev' THEN p.created_at END ASC,
			CASE WHEN $4 = 'prev' THEN p.profile_id END ASC,
			CASE WHEN $4 <> 'prev' THEN p.created_at END DESC,
			CASE WHEN $4 <> 'prev' THEN p.profile_id END DESC
		LIMIT $5;`,
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

var GetAllProfilesWithFilterBase = map[string]string{
	"postgres": `SELECT DISTINCT p.profile_id,
                p.user_id,
                p.org_handle,
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
		p.org_handle,
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
		AND p.profile_id != $1
		AND p.org_handle = $2;`,
}

var FetchReferencedProfiles = map[string]string{
	"postgres": `
		SELECT profile_id, reference_reason, profile_status 
		FROM profile_reference 
		WHERE reference_profile_id = $1;`,
}

var GetProfileByUserId = map[string]string{
	"postgres": `
		SELECT p.profile_id, p.user_id, p.created_at, p.updated_at,p.location, p.org_handle, p.list_profile, p.delete_profile, 
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
	"postgres": `INSERT INTO consent_categories (category_name, category_identifier, org_handle, purpose, destinations, is_mandatory)
				VALUES ($1, $2, $3, $4, $5, $6)`,
}

var UpsertDefaultIdentityDataCategory = map[string]string{
	"postgres": `INSERT INTO consent_categories (category_name, category_identifier, org_handle, purpose, destinations, is_mandatory)
				SELECT $1::VARCHAR, $2::VARCHAR, $3::VARCHAR, $4::VARCHAR, $5::TEXT[], TRUE
				WHERE NOT EXISTS (
					SELECT 1 FROM consent_categories WHERE org_handle = $3::VARCHAR AND is_mandatory = TRUE
				)`,
}

var GetAllConsentCategories = map[string]string{
	"postgres": `SELECT category_name, category_identifier, org_handle, purpose, destinations, is_mandatory FROM consent_categories`,
}

var GetConsentCategoryById = map[string]string{
	"postgres": `SELECT category_name, category_identifier, org_handle, purpose, destinations, is_mandatory FROM consent_categories WHERE category_identifier = $1`,
}

var GetConsentCategoryByName = map[string]string{
	"postgres": `SELECT category_name, category_identifier, org_handle, purpose, destinations, is_mandatory FROM consent_categories WHERE category_name = $1 AND org_handle = $2`,
}

var GetMandatoryConsentCategoryIdsByOrg = map[string]string{
	"postgres": `SELECT category_identifier FROM consent_categories WHERE org_handle = $1 AND is_mandatory = TRUE`,
}

var UpdateConsentCategory = map[string]string{
	"postgres": `UPDATE consent_categories SET category_name=$1, purpose=$2, destinations=$3 WHERE category_identifier=$4`,
}

var DeleteConsentCategory = map[string]string{
	"postgres": `DELETE FROM consent_categories WHERE category_identifier=$1`,
}

var InsertConsentCategoryAttribute = map[string]string{
	"postgres": `INSERT INTO consent_category_attributes (category_id, scope, attribute_name, attribute_id, application_identifier)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (category_id, scope, attribute_name, application_identifier) DO NOTHING`,
}

var GetConsentCategoryAttributesByCategoryId = map[string]string{
	"postgres": `SELECT scope, attribute_name, attribute_id, application_identifier FROM consent_category_attributes WHERE category_id = $1`,
}

var DeleteConsentCategoryAttributesByCategoryId = map[string]string{
	"postgres": `DELETE FROM consent_category_attributes WHERE category_id = $1`,
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

var UpdateCookieStatusByCookieId = map[string]string{
	"postgres": `UPDATE profile_cookies SET is_active = $1 WHERE cookie_id = $2`,
}

var DeleteCookieById = map[string]string{
	"postgres": `DELETE FROM profile_cookies WHERE cookie_id = $1`,
}

var DeleteCookieByProfileId = map[string]string{
	"postgres": `DELETE FROM profile_cookies WHERE profile_id = $1`,
}

var DeleteInactiveCookies = map[string]string{
	"postgres": `DELETE FROM cookie_profiles WHERE cookie_id IN (SELECT cookie_id FROM cookie_profiles 
                                                                 WHERE is_active = false LIMIT $1)`,
}

var GetOrgConfigurations = map[string]string{
	"postgres": `SELECT config, value FROM cds_config WHERE org_handle = $1`,
}

var UpdateOrgConfiguration = map[string]string{
	"postgres": `INSERT INTO cds_config (org_handle, config, value) 
                 VALUES ($1, $2, $3) 
                 ON CONFLICT (org_handle, config) 
                 DO UPDATE SET value = EXCLUDED.value`,
}

var GetOrgConfiguration = map[string]string{
	"postgres": `SELECT value FROM cds_config WHERE org_handle = $1 AND config = $2`,
}

var UpdateInitialSchemaSyncDoneConfig = map[string]string{
	"postgres": `INSERT INTO cds_config (org_handle, config, value) 
                 VALUES ($1, 'initial_schema_sync_done', $2) 
                 ON CONFLICT (org_handle, config) 
                 DO UPDATE SET value = EXCLUDED.value`,
}

var DeleteBlockingKeysSQL = map[string]string{
	"postgres": `DELETE FROM blocking_keys WHERE profile_id = $1`,
}

var DeleteBlockingKeysByAttributeSQL = map[string]string{
	"postgres": `DELETE FROM blocking_keys WHERE org_handle = $1 AND attribute_name = $2`,
}

var IRGetProfilesForOrgPaginated = map[string]string{
	"postgres": `SELECT profile_id, user_id, org_handle, traits, identity_attributes
				 FROM profiles
				 WHERE org_handle = $1 AND delete_profile = FALSE
				 ORDER BY profile_id
				 LIMIT $2 OFFSET $3`,
}

var IRGetProfilesByIDs = map[string]string{
	"postgres": `SELECT p.profile_id, p.user_id, p.org_handle, p.traits, p.identity_attributes,
				        pr.reference_profile_id
				 FROM profiles p
				 LEFT JOIN profile_reference pr ON p.profile_id = pr.profile_id
				 WHERE p.profile_id IN (%s) AND p.delete_profile = FALSE`,
}

var IRInsertBlockingKeys = map[string]string{
	"postgres": `INSERT INTO blocking_keys (key_id, profile_id, org_handle, attribute_name, key_value)
				 VALUES %s ON CONFLICT DO NOTHING`,
}

var IRFindCandidateIDsByKeys = map[string]string{
	"postgres": `SELECT DISTINCT profile_id FROM blocking_keys
				 WHERE org_handle = $1 AND attribute_name = $2 AND key_value IN (%s)
				   AND profile_id != $%d LIMIT $%d`,
}

var IRInsertReviewTask = map[string]string{
	"postgres": `INSERT INTO review_tasks (id, org_handle, incoming_profile_id, candidate_profile_id, match_score, status, score_breakdown)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)
				 ON CONFLICT (incoming_profile_id, candidate_profile_id)
				 DO UPDATE SET match_score = $5, score_breakdown = $7, status = $6
				 WHERE review_tasks.status = 'PENDING'`,
}

// IRMirrorReviewTaskExists checks whether a PENDING task exists for the reverse pair (candidate→incoming).
var IRMirrorReviewTaskExists = map[string]string{
	"postgres": `SELECT COUNT(*) FROM review_tasks
				 WHERE incoming_profile_id = $1 AND candidate_profile_id = $2 AND status = $3`,
}

// IRUpdateMirrorReviewTask flips the direction of a mirror task and refreshes its score.
var IRUpdateMirrorReviewTask = map[string]string{
	"postgres": `UPDATE review_tasks
				 SET incoming_profile_id = $1, candidate_profile_id = $2, match_score = $3, score_breakdown = $4
				 WHERE incoming_profile_id = $5 AND candidate_profile_id = $6 AND status = $7`,
}

// IRCancelRelatedReviewTasks cancels all PENDING tasks that reference either profile.
var IRCancelRelatedReviewTasks = map[string]string{
	"postgres": `UPDATE review_tasks
				 SET status = $1, resolved_at = now(), resolved_by = $2, resolution_notes = $3
				 WHERE id != $4 AND status = $5
				   AND (incoming_profile_id IN ($6, $7) OR candidate_profile_id IN ($6, $7))`,
}

// IRFindRelatedPendingReviewTasks finds incoming profile IDs of PENDING tasks affected by a cascade cancel.
var IRFindRelatedPendingReviewTasks = map[string]string{
	"postgres": `SELECT DISTINCT incoming_profile_id
				 FROM review_tasks
				 WHERE id != $1 AND status = $2
				   AND (incoming_profile_id IN ($3, $4) OR candidate_profile_id IN ($3, $4))`,
}

var IRGetReviewTaskByID = map[string]string{
	"postgres": `SELECT id, org_handle, incoming_profile_id, candidate_profile_id, match_score, status,
				        score_breakdown, created_at, resolved_at, resolved_by, resolution_notes
				 FROM review_tasks
				 WHERE id = $1`,
}

var IRGetPendingReviewTasks = map[string]string{
	"postgres": `SELECT id, org_handle, incoming_profile_id, candidate_profile_id, match_score, status,
				        score_breakdown, created_at, resolved_at, resolved_by, resolution_notes
				 FROM review_tasks
				 WHERE org_handle = $1 AND status = $2
				 ORDER BY created_at DESC
				 LIMIT $3`,
}

var IRCountPendingReviewTasks = map[string]string{
	"postgres": `SELECT COUNT(*) FROM review_tasks WHERE org_handle = $1 AND status = $2`,
}

var IRGetPendingReviewTasksByProfile = map[string]string{
	"postgres": `SELECT id, org_handle, incoming_profile_id, candidate_profile_id, match_score, status,
				        score_breakdown, created_at, resolved_at, resolved_by, resolution_notes
				 FROM review_tasks
				 WHERE org_handle = $1 AND status = $2
				   AND (incoming_profile_id = $3)
				 ORDER BY match_score DESC
				 LIMIT $4`,
}

var IRCountPendingReviewTasksByProfile = map[string]string{
	"postgres": `SELECT COUNT(*) FROM review_tasks WHERE org_handle = $1 AND status = $2
				   AND (incoming_profile_id = $3 OR candidate_profile_id = $3)`,
}

var IRUpdateReviewTaskStatus = map[string]string{
	"postgres": `UPDATE review_tasks
				 SET status = $1, resolved_at = now(), resolved_by = $2, resolution_notes = $3
				 WHERE id = $4`,
}

var IRInsertRejectionPair = map[string]string{
	"postgres": `INSERT INTO rejection_pairs (id, org_handle, profile_id_1, profile_id_2, rejected_by)
				 VALUES ($1, $2, $3, $4, $5)
				 ON CONFLICT (profile_id_1, profile_id_2) DO NOTHING`,
}

var IRGetRejectedProfileIDs = map[string]string{
	"postgres": `SELECT profile_id_1, profile_id_2 FROM rejection_pairs
				 WHERE org_handle = $1 AND (profile_id_1 = $2 OR profile_id_2 = $2)`,
}

var IRDeleteRejectionPairsForProfile = map[string]string{
	"postgres": `DELETE FROM rejection_pairs WHERE org_handle = $1 AND (profile_id_1 = $2 OR profile_id_2 = $2)`,
}

var IRInsertMergeAuditLog = map[string]string{
	"postgres": `INSERT INTO merge_audit_log (id, org_handle, primary_profile_id, secondary_profile_id, merge_type, match_score, merged_by)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
}
