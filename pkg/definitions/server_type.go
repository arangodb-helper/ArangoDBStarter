//
// DISCLAIMER
//
// Copyright 2017 ArangoDB GmbH, Cologne, Germany
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

package definitions

import (
	"fmt"
	"time"
)

// ServerType specifies the types of database servers.
type ServerType string

const (
	ServerTypeUnknown          = "unknown"
	ServerTypeCoordinator      = "coordinator"
	ServerTypeDBServer         = "dbserver"
	ServerTypeDBServerNoResign = "dbserver_noresign"
	ServerTypeAgent            = "agent"
	ServerTypeSingle           = "single"
	ServerTypeResilientSingle  = "resilientsingle"
	ServerTypeSyncMaster       = "syncmaster"
	ServerTypeSyncWorker       = "syncworker"
)

// String returns a string representation of the given ServerType.
func (s ServerType) String() string {
	return string(s)
}

// PortOffset returns the offset from a peer base port for the given type of server.
func (s ServerType) PortOffset() int {
	switch s {
	case ServerTypeCoordinator, ServerTypeSingle, ServerTypeResilientSingle:
		return PortOffsetCoordinator
	case ServerTypeDBServer, ServerTypeDBServerNoResign:
		return PortOffsetDBServer
	case ServerTypeAgent:
		return PortOffsetAgent
	case ServerTypeSyncMaster:
		return PortOffsetSyncMaster
	case ServerTypeSyncWorker:
		return PortOffsetSyncWorker
	default:
		panic(fmt.Sprintf("Unknown ServerType: %s", string(s)))
	}
}

// InitialStopTimeout returns initial delay for process stopping
func (s ServerType) InitialStopTimeout() time.Duration {
	switch s {
	case ServerTypeDBServer, ServerTypeSingle, ServerTypeResilientSingle:
		return 3 * time.Second
	default:
		return time.Second
	}
}

// ProcessType returns the type of process needed to run a server of given type.
func (s ServerType) ProcessType() ProcessType {
	switch s {
	case ServerTypeSyncMaster, ServerTypeSyncWorker:
		return ProcessTypeArangoSync
	default:
		return ProcessTypeArangod
	}
}

// ExpectedServerRole returns the expected `role` & `mode` value resulting from a request to `_admin/server/role`.
func (s ServerType) ExpectedServerRole() (string, string) {
	switch s {
	case ServerTypeCoordinator:
		return "COORDINATOR", ""
	case ServerTypeSingle:
		return "SINGLE", ""
	case ServerTypeResilientSingle:
		return "SINGLE", "resilient"
	case ServerTypeDBServer, ServerTypeDBServerNoResign:
		return "PRIMARY", ""
	case ServerTypeAgent:
		return "AGENT", ""
	case ServerTypeSyncMaster, ServerTypeSyncWorker:
		return "", ""
	default:
		panic(fmt.Sprintf("Unknown ServerType: %s", string(s)))
	}
}

// GetName returns readable name for the specific type of server.
func (s ServerType) GetName() string {
	switch s {
	case ServerTypeAgent:
		return "agent"
	case ServerTypeDBServer:
		return "dbserver"
  case ServerTypeDBServerNoResign:
		return "dbserver"
	case ServerTypeCoordinator:
		return "coordinator"
	case ServerTypeSingle, ServerTypeResilientSingle:
		return "single server"
	case ServerTypeSyncMaster:
		return "sync master"
	case ServerTypeSyncWorker:
		return "sync worker"
	}

	return ""
}

func AllServerTypes() []ServerType {
	return []ServerType{
		ServerTypeCoordinator,
		ServerTypeDBServer,
    ServerTypeDBServerNoResign,
		ServerTypeAgent,
		ServerTypeSingle,
		ServerTypeResilientSingle,
		ServerTypeSyncMaster,
		ServerTypeSyncWorker,
	}
}
