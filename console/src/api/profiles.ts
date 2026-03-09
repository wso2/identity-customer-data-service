import type {
  Profile,
  ProfileCreateRequest,
  ProfileListResponse,
  ProfilePatchRequest,
} from '../models/profile';
import { request } from './client';

export function getProfiles(limit = 15, cursor = ''): Promise<ProfileListResponse> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (cursor) params.set('cursor', cursor);
  return request<ProfileListResponse>(`/profiles?${params}`);
}

export function getProfile(id: string): Promise<Profile> {
  return request<Profile>(`/profiles/${id}`);
}

export function createProfile(body: ProfileCreateRequest): Promise<Profile> {
  return request<Profile>('/profiles', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export function patchProfile(id: string, body: ProfilePatchRequest): Promise<Profile> {
  return request<Profile>(`/profiles/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

export function deleteProfile(id: string): Promise<void> {
  return request<void>(`/profiles/${id}`, { method: 'DELETE' });
}
