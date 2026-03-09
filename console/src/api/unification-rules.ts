import type {
  UnificationRule,
  UnificationRuleCreateRequest,
  UnificationRuleUpdateRequest,
} from '../models/unification-rule';
import { request } from './client';

export function getUnificationRules(): Promise<UnificationRule[]> {
  return request<UnificationRule[]>('/unification-rules');
}

export function addUnificationRule(body: UnificationRuleCreateRequest): Promise<UnificationRule> {
  return request<UnificationRule>('/unification-rules', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export function patchUnificationRule(id: string, body: UnificationRuleUpdateRequest): Promise<UnificationRule> {
  return request<UnificationRule>(`/unification-rules/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

export function deleteUnificationRule(id: string): Promise<void> {
  return request<void>(`/unification-rules/${id}`, { method: 'DELETE' });
}
