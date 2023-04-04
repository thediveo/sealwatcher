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

package sealwatcher

import (
	"context"

	"github.com/cenkalti/backoff/v4"
	"github.com/containers/podman/v4/pkg/bindings"
	engineclient "github.com/thediveo/sealwatcher/podman"
	"github.com/thediveo/whalewatcher/watcher"
)

// Type ID of the container engine handled by this watcher.
const Type = engineclient.Type

// PodLabelName is the label key for the pod name in case a container belongs to
// a pod.
const PodLabelName = engineclient.PodLabelName

// InfraLabelName is the label key that is present on “infrastructure”
// containers only; the label value is irrelevant and must not be relied upon.
const InfraLabelName = engineclient.InfraLabelName

// New returns a [watcher.Watcher] for keeping track of the currently alive
// containers, optionally with the composer projects they're associated with.
//
// When the podmansock parameter is left empty then Podman's usual client
// defaults apply, such as trying to pick it up from the environment or falling
// back to the local host's "unix:///run/podman/podman.sock".
//
// If the backoff is nil then the backoff defaults to backoff.StopBackOff, that
// is, any failed operation will never be retried.
//
// Finally, Podman engine client-specific options can be passed in.
func New(podmansock string, buggeroff backoff.BackOff, opts ...engineclient.NewOption) (watcher.Watcher, error) {
	conn, err := bindings.NewConnection(context.Background(), podmansock)
	if err != nil {
		return nil, err
	}
	return watcher.New(engineclient.NewPodmanWatcher(conn, opts...), buggeroff), nil
}
