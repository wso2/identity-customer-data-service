import type { AdminConfig } from '../models/admin-config';
import { request } from './client';

export function getAdminConfig(): Promise<AdminConfig> {
  return request<AdminConfig>('/config');
}

export function updateAdminConfig(body: AdminConfig): Promise<AdminConfig> {
  return request<AdminConfig>('/config', {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}
