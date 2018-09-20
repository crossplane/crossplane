/*
Copyright 2018 The Conductor Authors.

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

package util

import (
	"crypto/rand"
	"encoding/base64"
)

func GeneratePassword(dataLen int) (string, error) {
	randData := make([]byte, dataLen)
	if _, err := rand.Read(randData); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(randData), nil
}
