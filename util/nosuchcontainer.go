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

/*
Package util provides convenience utilities for working with the Podman REST API
client.
*/
package util

import (
	"net/http"

	"github.com/containers/podman/v3/pkg/errorhandling"
)

// IsNoSuchContainerErr returns true if the given error is a 404 error response
// and the cause is "no such container".
func IsNoSuchContainerErr(err error) bool {
	em, ok := err.(*errorhandling.ErrorModel)
	if !ok {
		return false
	}
	return (em.ResponseCode == http.StatusNotFound) && (em.Because == "no such container")
}
