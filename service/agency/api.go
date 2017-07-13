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

package agency

import (
	"context"
	"time"
)

// API abstracts the API of the ArangoDB agency
type API interface {
	// ReadKey reads the value of a given key in the agency.
	ReadKey(ctx context.Context, key []string) (interface{}, error)

	// WriteKeyIfEmpty writes the given value with the given key only if the key was empty before.
	WriteKeyIfEmpty(ctx context.Context, key []string, value interface{}, ttl time.Duration) error

	// WriteKeyIfEqualTo writes the given new value with the given key only if the existing value for that key equals
	// to the given old value.
	WriteKeyIfEqualTo(ctx context.Context, key []string, newValue, oldValue interface{}, ttl time.Duration) error
}