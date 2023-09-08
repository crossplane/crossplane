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

// Package password contains a simple password generator.
package password

import (
	"crypto/rand"
	"math/big"
)

// Settings for password generation.
type Settings struct {
	// CharacterSet of allowed password characters.
	CharacterSet string

	// Length of generated passwords.
	Length int
}

// Default password generation settings.
var Default = Settings{
	CharacterSet: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
	Length:       27,
}

// Generate a random, 27 character password that may consist of lowercase
// letters, uppercase letters, or numbers.
func Generate() (string, error) {
	return Default.Generate()
}

// Generate a password.
func (s Settings) Generate() (string, error) {
	pw := make([]byte, s.Length)
	for i := 0; i < s.Length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(s.CharacterSet))))
		if err != nil {
			return "", err
		}
		pw[i] = s.CharacterSet[n.Int64()]
	}
	return string(pw), nil
}
