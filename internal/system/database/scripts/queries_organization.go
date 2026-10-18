/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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

// Statements for organization management. The domains are:
//
//	CDS-ORG  organizations
//	CDS-SHR  cds_share_policy, cds_share_policy_target

const organizationColumns = `org_id, org_handle, org_name, parent_org_id, primary_org_id, status, created_at,
	updated_at`

const organizationColumnsOfO = `o.org_id, o.org_handle, o.org_name, o.parent_org_id, o.primary_org_id, o.status,
	o.created_at, o.updated_at`

var UpsertOrganization = newQuery("CDS-ORG-01",
	`INSERT INTO organizations (org_id, org_handle, org_name, parent_org_id, primary_org_id, status, created_at,
	updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
	ON CONFLICT (org_id) DO UPDATE SET org_handle = excluded.org_handle, org_name = excluded.org_name,
	parent_org_id = excluded.parent_org_id, primary_org_id = excluded.primary_org_id, status = excluded.status,
	updated_at = excluded.updated_at`)

var GetOrganizationById = newQuery("CDS-ORG-02",
	`SELECT `+organizationColumns+` FROM organizations WHERE org_id = $1`)

var GetOrganizationByHandle = newQuery("CDS-ORG-03",
	`SELECT `+organizationColumns+` FROM organizations WHERE org_handle = $1`)

var GetOrganizationsByPrimaryOrg = newQuery("CDS-ORG-04",
	`SELECT `+organizationColumns+` FROM organizations WHERE primary_org_id = $1 ORDER BY org_id`)

var UpdateOrganizationStatus = newQuery("CDS-ORG-05",
	`UPDATE organizations SET status = $1, updated_at = $2 WHERE org_id = $3`)

// chainOf is the ancestor chain of the organization $1, the organization included. hops is the
// distance from the organization.
const chainOf = `WITH RECURSIVE chain (org_id, parent_org_id, hops) AS (
	SELECT org_id, parent_org_id, 0 FROM organizations WHERE org_id = $1
	UNION ALL
	SELECT o.org_id, o.parent_org_id, c.hops + 1 FROM organizations o JOIN chain c ON o.org_id = c.parent_org_id)
	`

// GetOrganizationChain returns the organization $1 and its ancestors, the primary organization
// first.
var GetOrganizationChain = newQuery("CDS-ORG-07",
	chainOf+`SELECT `+organizationColumnsOfO+` FROM organizations o JOIN chain c ON c.org_id = o.org_id
	ORDER BY c.hops DESC`)

// GetOrganizationsByIds returns the organizations with the IDs. The %s is the IN list.
var GetOrganizationsByIds = newQuery("CDS-ORG-08",
	`SELECT `+organizationColumns+` FROM organizations WHERE org_id IN (%s)`)

var InsertSharePolicy = newQuery("CDS-SHR-01",
	`INSERT INTO cds_share_policy (policy_id, resource_type, resource_id, owning_org_id, initiating_org_id)
	VALUES ($1, $2, $3, $4, $5)`)

const sharePolicyColumns = `p.policy_id, p.resource_type, p.resource_id, p.owning_org_id, p.initiating_org_id`

var GetSharePolicyById = newQuery("CDS-SHR-02",
	`SELECT `+sharePolicyColumns+` FROM cds_share_policy p WHERE p.policy_id = $1`)

var GetSharePolicy = newQuery("CDS-SHR-03",
	`SELECT `+sharePolicyColumns+` FROM cds_share_policy p
	WHERE p.resource_type = $1 AND p.resource_id = $2 AND p.initiating_org_id = $3`)

var GetSharePolicyTargets = newQuery("CDS-SHR-06",
	`SELECT t.policy_id, t.target_scope, t.target_org_id FROM cds_share_policy_target t WHERE t.policy_id = $1`)

// InsertSharePolicyTargets stores the targets of a policy. The %s is the VALUES list.
var InsertSharePolicyTargets = newQuery("CDS-SHR-07",
	`INSERT INTO cds_share_policy_target (policy_id, target_scope, target_org_id) VALUES %s
	ON CONFLICT DO NOTHING`)

var DeleteSharePolicyTargets = newQuery("CDS-SHR-08",
	`DELETE FROM cds_share_policy_target WHERE policy_id = $1`)

var DeleteSharePolicy = newQuery("CDS-SHR-09",
	`DELETE FROM cds_share_policy WHERE policy_id = $1`)

var GetSharePolicyIdsByResource = newQuery("CDS-SHR-10",
	`SELECT policy_id FROM cds_share_policy WHERE resource_type = $1 AND resource_id = $2`)

// reachingOrg selects the targets that reach the organization $1.
const reachingOrg = chainOf + `SELECT ` + sharePolicyColumns + `, t.target_scope, t.target_org_id
	FROM cds_share_policy_target t
	JOIN chain c ON c.org_id = t.target_org_id
	JOIN cds_share_policy p ON p.policy_id = t.policy_id
	WHERE ((t.target_scope = 'ORG' AND t.target_org_id = $1)
		OR t.target_scope = 'ORG_SUBTREE'
		OR (t.target_scope = 'ALL_CHILDREN' AND t.target_org_id <> $1))`

// GetSharePoliciesReachingOrg returns the share policies whose targets reach the organization $1,
// with the targets that reach it.
var GetSharePoliciesReachingOrg = newQuery("CDS-SHR-11",
	reachingOrg+` AND p.resource_type <> 'ORGANIZATION_ACCESS' ORDER BY p.policy_id`)

// GetOrgAccessPoliciesReachingOrg returns the organization access policies whose targets reach
// the organization $1.
var GetOrgAccessPoliciesReachingOrg = newQuery("CDS-SHR-12",
	reachingOrg+` AND p.resource_type = 'ORGANIZATION_ACCESS'`)

const attributeColumns = `ps.attribute_id, ps.attribute_name, ps.scope, ps.display_name, ps.value_type,
	ps.merge_strategy, ps.mutability, ps.application_identifier, ps.multi_valued,
	CAST(ps.sub_attributes AS TEXT) AS sub_attributes, CAST(ps.canonical_values AS TEXT) AS canonical_values,
	ps.org_handle, o.org_id, c.hops AS owner_hops`

// GetSchemaAttributesOfChain returns the attributes of the organization $1 and of its ancestors.
var GetSchemaAttributesOfChain = newQuery("CDS-SHR-15",
	chainOf+`SELECT `+attributeColumns+` FROM profile_schema ps JOIN organizations o ON o.org_handle = ps.org_handle
	JOIN chain c ON c.org_id = o.org_id`)

// GetUnificationRulesOfChain returns the rules of the organization $1 and of its ancestors, with
// the number of levels from the organization to the owner.
var GetUnificationRulesOfChain = newQuery("CDS-SHR-16",
	chainOf+`SELECT r.rule_id, r.org_handle, r.rule_name, r.property_name, r.property_id, r.priority, r.is_active,
	r.created_at, r.updated_at, o.org_id, c.hops AS owner_hops
	FROM unification_rules r JOIN organizations o ON o.org_handle = r.org_handle
	JOIN chain c ON c.org_id = o.org_id`)
