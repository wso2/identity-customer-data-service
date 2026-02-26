const BASE = '/t/carbon.super/cds/api/v1';

async function request(method, path, body = null) {
  const opts = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  if (body) opts.body = JSON.stringify(body);

  const res = await fetch(`${BASE}${path}`, opts);
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.description || err.message || `HTTP ${res.status}`);
  }
  if (res.status === 204) return null;
  return res.json();
}

// ─── Profiles ──────────────────────────────────────────
export const getProfiles = (pageSize = 20, cursor = '', attrs = 'identity_attributes,traits') => {
  let url = `/profiles?page_size=${pageSize}&attributes=${encodeURIComponent(attrs)}`;
  if (cursor) url += `&cursor=${encodeURIComponent(cursor)}`;
  return request('GET', url);
};
export const getProfile = (id) =>
  request('GET', `/profiles/${id}?includeApplicationData=true&application_identifier=*`);
export const createProfile = (data) => request('POST', '/profiles', data);
export const patchProfile = (id, data) => request('PATCH', `/profiles/${id}`, data);
export const deleteProfile = (id) => request('DELETE', `/profiles/${id}`);
export const getProfileConsents = (id) => request('GET', `/profiles/${id}/consents`);
export const updateProfileConsents = (id, consents) =>
  request('PUT', `/profiles/${id}/consents`, consents);

// ─── Profile Schema ────────────────────────────────────
export const getSchema = () => request('GET', '/profile-schema');
export const getSchemaScope = (scope) => request('GET', `/profile-schema/${scope}`);
export const addSchemaAttributes = (scope, attrs) =>
  request('POST', `/profile-schema/${scope}`, Array.isArray(attrs) ? attrs : [attrs]);
export const getSchemaAttribute = (scope, attrId) =>
  request('GET', `/profile-schema/${scope}/${attrId}`);
export const patchSchemaAttribute = (scope, attrId, data) =>
  request('PATCH', `/profile-schema/${scope}/${attrId}`, data);
export const deleteSchemaAttribute = (scope, attrId) =>
  request('DELETE', `/profile-schema/${scope}/${attrId}`);
export const deleteSchemaScope = (scope) => request('DELETE', `/profile-schema/${scope}`);

// ─── Unification Rules ─────────────────────────────────
export const getUnificationRules = () => request('GET', '/unification-rules');
export const getUnificationRule = (id) => request('GET', `/unification-rules/${id}`);
export const addUnificationRule = (data) => request('POST', '/unification-rules', data);
export const patchUnificationRule = (id, data) =>
  request('PATCH', `/unification-rules/${id}`, data);
export const deleteUnificationRule = (id) => request('DELETE', `/unification-rules/${id}`);

// ─── Identity Resolution ───────────────────────────────
export const searchProfiles = (body) =>
  request('POST', '/identity-resolution/search', body);
export const mergeProfiles = (body) =>
  request('POST', '/identity-resolution/merge', body);
export const getReviewTasks = (pageSize = 100) =>
  request('GET', `/identity-resolution/review-tasks?page_size=${pageSize}`);
export const getReviewTasksByProfile = (profileId, pageSize = 50) =>
  request('GET', `/identity-resolution/review-tasks?profile_id=${profileId}&page_size=${pageSize}`);
export const resolveReviewTask = (taskId, decision, notes = '') =>
  request('POST', `/identity-resolution/review-tasks/${taskId}/resolve`, { decision, notes });

// ─── Admin Config ──────────────────────────────────────
export const getAdminConfig = () => request('GET', '/config');
export const updateAdminConfig = (data) => request('PATCH', '/config', data);
