import type { SchemaAttribute, SchemaAttributeRequest, SchemaResponse, SchemaScope } from '../models/schema';
import { request } from './client';

export function getSchema(): Promise<SchemaResponse> {
  return request<SchemaResponse>('/profile-schema');
}

export function getSchemaScope(scope: SchemaScope): Promise<SchemaResponse> {
  return request<SchemaResponse>(`/profile-schema/${scope}`);
}

export function addSchemaAttributes(scope: SchemaScope, body: SchemaAttributeRequest): Promise<SchemaAttribute> {
  return request<SchemaAttribute>(`/profile-schema/${scope}`, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export function patchSchemaAttribute(scope: SchemaScope, id: string, body: Partial<SchemaAttributeRequest>): Promise<SchemaAttribute> {
  return request<SchemaAttribute>(`/profile-schema/${scope}/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

export function deleteSchemaAttribute(scope: SchemaScope, id: string): Promise<void> {
  return request<void>(`/profile-schema/${scope}/${id}`, { method: 'DELETE' });
}
