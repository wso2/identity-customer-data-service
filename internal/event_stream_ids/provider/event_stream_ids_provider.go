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
	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/service"
)

// EventStreamIdProviderInterface defines the interface for the enrichment rule provider.
type EventStreamIdProviderInterface interface {
	GetEventStreamIdService() service.EventStreamIdServiceInterface
}

// EventStreamIdProvider is the default implementation of the EventStreamIdProviderInterface.
type EventStreamIdProvider struct{}

// EventStreamIdProviderInterface creates a new instance of EnrichmentRuleProvider.
func NewEventStreamIdProvider() EventStreamIdProviderInterface {

	return &EventStreamIdProvider{}
}

// GetEventStreamIdService returns the enrichment rule service instance.
func (ap *EventStreamIdProvider) GetEventStreamIdService() service.EventStreamIdServiceInterface {

	return service.GetEventStreamIdService()
}
