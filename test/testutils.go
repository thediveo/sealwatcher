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

package test

import (
	"context"

	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/bindings/images"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/thediveo/sealwatcher/v2/util"

	g "github.com/onsi/gomega"
	s "github.com/thediveo/success"
)

// TargetContainerStatus is the status a newly created container should reach
// (Created, Running, Paused).
type TargetContainerStatus int

// Target status a newly created container should reach.
const (
	Created TargetContainerStatus = iota // just create the container
	Running                              // start the container
	Paused                               // start, then pause the container
)

// NewContainerDescription describes the properties of a container to be newly
// created.
type NewContainerDescription struct {
	Name   string
	Status TargetContainerStatus
	Labels map[string]string
}

// NewContainerOption sets an optional specification property when creating a
// new container.
type NewContainerOption func(spec *specgen.SpecGenerator)

// WithImage specifies the image from which the new container is to be created.
func WithImage(image string) NewContainerOption {
	return func(spec *specgen.SpecGenerator) {
		spec.Image = image
	}
}

// WithCommand specifies the command to start inside the new container.
func WithCommand(command []string) NewContainerOption {
	return func(spec *specgen.SpecGenerator) {
		spec.Command = command
	}
}

// AsPrivileged requests a privileged container.
func AsPrivileged() NewContainerOption {
	return func(spec *specgen.SpecGenerator) {
		spec.Privileged = true
	}
}

// OfPod sets the name of the pod to which the newly created container belongs.
func OfPod(pod string) NewContainerOption {
	return func(spec *specgen.SpecGenerator) {
		spec.Pod = pod
	}
}

// NewContainer creates a new container and optionally starts it, depending on
// the target container state configured in the passed container description.
// NewContainer only returns on success. Otherwise, a failed Gomega assertion
// will be raised.
func NewContainer(conn context.Context, desc NewContainerDescription, options ...NewContainerOption) (id string) {
	spec := &specgen.SpecGenerator{}
	spec.Name = desc.Name
	spec.Labels = desc.Labels
	for _, opt := range options {
		opt(spec)
	}

	if spec.Image == "" && spec.Rootfs == "" {
		spec.Image = "docker.io/library/busybox"
		spec.Command = []string{"/bin/sh", "-c", "i=60; while [ $i -ne 0 ]; do sleep 1; i=$(($i-1)); done"}
	}

	policy := "missing"
	g.Expect(images.Pull(conn, spec.Image, &images.PullOptions{Policy: &policy})).Error().NotTo(g.HaveOccurred())

	resp := s.Successful(containers.CreateWithSpec(conn, spec, nil))
	id = resp.ID
	if desc.Status == Created {
		return
	}
	g.Expect(containers.Start(conn, desc.Name, nil)).NotTo(g.HaveOccurred())
	if desc.Status == Running {
		return
	}
	g.Expect(containers.Pause(conn, desc.Name, nil)).NotTo(g.HaveOccurred())
	return
}

// RemoveContainer removes (forcefully, where necessary) the specified container
// by name. It will succeed and return even if the container currently doesn't
// exist. All other errors will raise a failed Gomega assertion.
func RemoveContainer(conn context.Context, name string) {
	force := true
	if _, err := containers.Remove(
		conn, name, &containers.RemoveOptions{Force: &force}); err != nil && !util.IsNoSuchContainerErr(err) {
		g.Expect(err).NotTo(g.HaveOccurred())
	}
}
