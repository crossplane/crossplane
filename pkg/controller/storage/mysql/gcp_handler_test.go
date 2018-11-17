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

package mysql

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestTranslateVersion(t *testing.T) {
	g := NewGomegaWithT(t)
	g.Expect(translateVersion("5.6")).To(Equal("MYSQL_5_6"))
	g.Expect(translateVersion("foo.bar.baz")).To(Equal("MYSQL_foo_bar_baz"))
}
