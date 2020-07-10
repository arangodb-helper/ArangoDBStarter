//
// DISCLAIMER
//
// Copyright 2018 ArangoDB GmbH, Cologne, Germany
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Copyright holder is ArangoDB GmbH, Cologne, Germany
//
// Author Ewout Prangsma
//

package service

import (
	"github.com/arangodb-helper/arangodb/pkg/definitions"
	driver "github.com/arangodb/go-driver"
)

type ConnectionType int

const (
	ConnectionTypeDatabase ConnectionType = iota
	ConnectionTypeAgency
)

// ClientBuilder is a callback used to create authenticated go-driver clients with or without
// follow-redirect.
type ClientBuilder func(endpoints []string, connectionType ConnectionType, serverType definitions.ServerType) (driver.Client, error)
