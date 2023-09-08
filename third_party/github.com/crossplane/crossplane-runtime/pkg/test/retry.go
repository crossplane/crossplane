/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// DefaultRetry is the recommended retry parameters for unit testing scenarios where a condition is being
// tested multiple times before it is expected to succeed.
var DefaultRetry = wait.Backoff{
	Steps:    500,
	Duration: 10 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
}
