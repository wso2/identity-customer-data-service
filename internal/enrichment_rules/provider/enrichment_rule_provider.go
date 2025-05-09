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

package provider

import (
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/service"
)

// EnrichmentRuleProviderInterface defines the interface for the enrichment rule provider.
type EnrichmentRuleProviderInterface interface {
	GetEnrichmentRuleService() service.EnrichmentRuleServiceInterface
}

// EnrichmentRuleProvider is the default implementation of the EnrichmentRuleProviderInterface.
type EnrichmentRuleProvider struct{}

// NewEnrichmentRuleProvider creates a new instance of EnrichmentRuleProvider.
func NewEnrichmentRuleProvider() EnrichmentRuleProviderInterface {

	return &EnrichmentRuleProvider{}
}

// GetEnrichmentRuleService returns the enrichment rule service instance.
func (ap *EnrichmentRuleProvider) GetEnrichmentRuleService() service.EnrichmentRuleServiceInterface {

	return service.GetEnrichmentRuleService()
}
