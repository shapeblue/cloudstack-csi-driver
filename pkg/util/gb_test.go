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

package util

import (
	"strconv"
	"testing"
)

func TestRoundUpBytesToGB(t *testing.T) {
	cases := []struct {
		b          int64
		expectedGb int64
	}{
		{100, 1},
		{3221225472, 3},
		{3000000000, 3},
		{50 * 1024 * 1024 * 1024, 50},
		{50*1024*1024*1024 - 1, 50},
		{50*1024*1024*1024 + 1, 51},
	}
	for _, c := range cases {
		t.Run(strconv.FormatInt(c.b, 10), func(t *testing.T) {
			gb := RoundUpBytesToGB(c.b)
			if gb != c.expectedGb {
				t.Errorf("%v bytes: expecting %v, got %v", c.b, c.expectedGb, gb)
			}
		})
	}
}

func TestGigaBytesToBytes(t *testing.T) {
	var gb int64 = 5
	b := GigaBytesToBytes(gb)
	var expectedBytes int64 = 5368709120
	if b != expectedBytes {
		t.Errorf("Expected %v, got %v", expectedBytes, b)
	}
	back := RoundUpBytesToGB(b)
	if back != gb {
		t.Errorf("Expected %v, got %v", gb, back)
	}
}
