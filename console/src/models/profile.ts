import type { Pagination } from './common';

export interface Meta {
  created_at: string;
  updated_at: string;
  location: string;
}

export interface Reference {
  profile_id: string;
  reason?: string;
}

export interface Profile {
  profile_id: string;
  user_id?: string;
  meta: Meta;
  identity_attributes?: Record<string, unknown>;
  traits?: Record<string, unknown>;
  application_data?: Record<string, Record<string, unknown>>;
  merged_to?: Reference | string;
  merged_from?: (Reference | string)[];
}

export interface ProfileListItem {
  profile_id: string;
  user_id?: string;
  meta: Meta;
  identity_attributes?: Record<string, unknown>;
  traits?: Record<string, unknown>;
  application_data?: Record<string, Record<string, unknown>>;
  merged_from?: (Reference | string)[];
}

export interface ProfileListResponse {
  profiles: ProfileListItem[];
  pagination: Pagination;
}

export interface ProfileCreateRequest {
  user_id?: string;
  identity_attributes?: Record<string, unknown>;
  traits?: Record<string, unknown>;
  application_data?: Record<string, Record<string, unknown>>;
}

export interface ProfilePatchRequest {
  identity_attributes?: Record<string, unknown>;
  traits?: Record<string, unknown>;
  application_data?: Record<string, Record<string, unknown>>;
}
