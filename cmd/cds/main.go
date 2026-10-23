/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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

// Command cds is the executable an installed CDS pack ships at its root.
//
// It owns the whole on-premises lifecycle: enabling CDS against an Identity
// Server, starting and stopping the runtime, and verifying the result. The
// container image runs cmd/server instead, where the orchestrator owns the
// lifecycle and only the runtime is wanted.
package main

import (
	"os"

	"github.com/wso2/identity-customer-data-service/internal/cli"
)

func main() { os.Exit(cli.Main(os.Args[1:])) }
