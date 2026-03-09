export interface UnificationRule {
  rule_id: string;
  rule_name: string;
  property_name: string;
  priority: number;
  is_active: boolean;
}

export interface UnificationRuleCreateRequest {
  rule_name: string;
  property_name: string;
  priority: number;
  is_active: boolean;
}

export interface UnificationRuleUpdateRequest {
  rule_name?: string;
  priority?: number;
  is_active?: boolean;
}
