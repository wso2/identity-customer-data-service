export type ValueType = 'string' | 'integer' | 'boolean' | 'date' | 'date_time' | 'epoch' | 'complex';
export type MergeStrategy = 'overwrite' | 'combine' | 'ignore';
export type Mutability = 'readWrite' | 'readOnly' | 'immutable' | 'writeOnly';
export type SchemaScope = 'identity_attributes' | 'traits' | 'application_data';

export interface SchemaAttribute {
  attribute_id: string;
  attribute_name: string;
  value_type: ValueType;
  merge_strategy: MergeStrategy;
  mutability: Mutability;
  multi_valued: boolean;
  application_identifier?: string;
}

export interface SchemaAttributeRequest {
  attribute_name: string;
  value_type: ValueType;
  merge_strategy: MergeStrategy;
  mutability: Mutability;
  multi_valued: boolean;
  application_identifier?: string;
}

export type SchemaResponse =
  | SchemaAttribute[]
  | Record<string, SchemaAttribute[]>;
