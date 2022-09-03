// Copyright 2022 Harald Albrecht.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"errors"
	"net/http"

	"github.com/containers/podman/v3/pkg/errorhandling"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("util", Ordered, func() {

	Context("IsNoSuchContainerErr", func() {

		It("doesn't match a nil error", func() {
			Expect(IsNoSuchContainerErr(nil)).To(BeFalse())
		})

		It("doesn't match an arbitrary podman REST API error", func() {
			err := &errorhandling.ErrorModel{
				ResponseCode: http.StatusServiceUnavailable,
				Because:      "I'm sorry, Dave. I'm afraid I can't do that.",
			}
			Expect(IsNoSuchContainerErr(err)).To(BeFalse())
		})

		It("doesn't match an arbitrary error", func() {
			Expect(IsNoSuchContainerErr(errors.New("42"))).To(BeFalse())
		})

		It("matches a no-such-container error", func() {
			err := &errorhandling.ErrorModel{
				ResponseCode: http.StatusNotFound,
				Because:      "no such container",
			}
			Expect(IsNoSuchContainerErr(err)).To(BeTrue())
		})

	})

})
