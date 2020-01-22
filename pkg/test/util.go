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
	"context"
	"io/ioutil"
	"time"

	"github.com/ghodss/yaml"
)

func WaitFor(ctx context.Context, interval time.Duration, check func(chan error)) error {
	timeout, cancel := context.WithTimeout(ctx, interval)
	defer cancel()

	ch := make(chan error, 1)
	stop := make(chan bool, 1)

	// TODO this needs to have some synchronization between the check and the loop running the check,
	// so that the loop does not run again if the check passes. Currently the check is able to keep going
	// until the context is canceled, which could be in the middle of a check.
	// It's either that or set the expectation that the check is an idempotent function which can be
	// killed at any time, and the test should be okay with that.
	go func() {
		for {
			select {
			case <-stop:
				break
			default:
				check(ch)
			}
		}
	}()

	select {
	case <-timeout.Done():
		close(stop)
		return timeout.Err()
	case err := <-ch:
		close(stop)
		return err
	}
}

func UnmarshalFromFile(path string, obj interface{}) error {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(dat, obj); err != nil {
		return err
	}
	return nil
}
