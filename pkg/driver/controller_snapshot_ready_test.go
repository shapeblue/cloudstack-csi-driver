//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//

package driver

import "testing"

func TestIsSnapshotReady(t *testing.T) {
	cases := map[string]bool{
		"Allocated":        false,
		"Creating":         false,
		"CreatedOnPrimary": false,
		"BackingUp":        false,
		"Copying":          false,
		"BackedUp":         true,
		"":                 true,
	}
	for state, want := range cases {
		if got := isSnapshotReady(state); got != want {
			t.Errorf("isSnapshotReady(%q) = %v, want %v", state, got, want)
		}
	}
}
