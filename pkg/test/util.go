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

// WaitFor is for waiting until a check returns that the check worked. It times out after the specified interval.
func WaitFor(ctx context.Context, interval time.Duration, check func(chan error)) error {
	timeout, cancel := context.WithTimeout(ctx, interval)
	defer cancel()

	ch := make(chan error, 1)
	stop := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-stop:
				return
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

// UnmarshalFromFile reads a yaml file and unmarshals it into an object
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
