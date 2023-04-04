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

package podman

import (
	"context"
	"sync"
	"time"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/bindings/pods"
	"github.com/containers/podman/v4/pkg/bindings/system"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/jellydator/ttlcache/v3"
	"github.com/thediveo/sealwatcher/v2/util"
	"github.com/thediveo/whalewatcher"
	"github.com/thediveo/whalewatcher/engineclient"
	"github.com/thediveo/whalewatcher/engineclient/moby"
	"github.com/thediveo/wye"
)

// Type specifies this container engine's type identifier.
const Type = "podman.io"

// Podman-specific "annotation" labels.
const (
	PodmanAnnotation = "io.github.thediveo/podman/"

	PodLabelName   = PodmanAnnotation + "podname" // name of pod if applicable
	PodIDName      = PodmanAnnotation + "podid"   // ID of pod if applicable
	InfraLabelName = PodmanAnnotation + "infra"   // present only if container is an infra container
)

// PodmanWatcher is a Podman EngineClient for interfacing the generic whale
// watching with podman daemons. There, I've said the bad word: "daemon". Never
// say "daemon" in the podman world. The podman creators even implemented a
// process dead (line) just to justify not having to call it "daemon" because it
// doesn't run constantly in the background. Unless someone watches a podman.
type PodmanWatcher struct { //revive:disable-line:exported
	pid      int                             // optional engine PID when known.
	podman   context.Context                 // (minimal) moby engine API client ... which is actually a context?!
	packer   engineclient.RucksackPacker     // optional Rucksack packer for app-specific container information.
	podcache *ttlcache.Cache[string, string] // pod ID->name TTL cache

	vmu     sync.Mutex
	version string // cached version information
}

// Make sure that the EngineClient interface is fully implemented.
var _ (engineclient.EngineClient) = (*PodmanWatcher)(nil)
var _ (engineclient.Trialer) = (*PodmanWatcher)(nil)

// NewPodmanWatcher returns a new PodmanWatcher using the specified podman
// connection; typically, you would want to use this lower-level constructor
// only in unit tests and instead use sealwatcher.New instead in most use
// cases.
func NewPodmanWatcher(podman context.Context, opts ...NewOption) *PodmanWatcher {
	pw := &PodmanWatcher{
		podman:   podman,
		podcache: ttlcache.New(ttlcache.WithTTL[string, string](1 * time.Minute)),
	}
	for _, opt := range opts {
		opt(pw)
	}
	go pw.podcache.Start()
	return pw
}

// NewOption represents options to [NewPodmanWatcher] when creating new watchers
// keeping eyes on Podman daemons.
type NewOption func(*PodmanWatcher)

// WithPID sets the engine's PID when known.
func WithPID(pid int) NewOption {
	return func(pw *PodmanWatcher) {
		pw.pid = pid
	}
}

// WithRucksackPacker sets the Rucksack packer that adds application-specific
// container information based on the inspected container data. The specified
// Rucksack packer gets passed the inspection data in form of a Docker client
// types.ContainerJSON.
func WithRucksackPacker(packer engineclient.RucksackPacker) NewOption {
	return func(pw *PodmanWatcher) {
		pw.packer = packer
	}
}

// ID returns the (more or less) unique engine identifier; the exact format is
// engine-specific. In case of Podman there is no genuine engine ID due to
// Podman's architecture. So we simply use the API endpoint path as the ID.
func (pw *PodmanWatcher) ID(svcctx context.Context) string {
	return pw.API() // ...avoid the Info roundtrips.
}

// Type returns the type identifier for this container engine.
func (pw *PodmanWatcher) Type() string { return Type }

// Version information about this Podman engine.
func (pw *PodmanWatcher) Version(svcctx context.Context) string {
	// Turns out that the Podman Info service is extremely slow, so we need to
	// cache the Podman engine version information.
	pw.vmu.Lock()
	defer pw.vmu.Unlock()
	if pw.version == "" {
		pw.fetchVersionUnderLock(svcctx)
	}
	return pw.version
}

// Try queries the version of the Podman service and caches the result.
func (pw *PodmanWatcher) Try(svcctx context.Context) error {
	pw.vmu.Lock()
	defer pw.vmu.Unlock()
	return pw.fetchVersionUnderLock(svcctx)
}

// fetchVersionUnderLock unconditionally fetches the Podman engine version and
// updates our cached version information. If there is an error fetching the
// version, then the cached version is set to "unknown" and an error returned.
func (pw *PodmanWatcher) fetchVersionUnderLock(svcctx context.Context) error {
	ctx, release := pw.y(svcctx)
	defer release()

	info, err := system.Version(ctx, nil)
	if err != nil {
		pw.version = "unknown"
		return err
	}
	pw.version = info.Server.Version
	return nil
}

// API returns the container engine API path.
func (pw *PodmanWatcher) API() string {
	client, err := bindings.GetClient(pw.podman)
	if err != nil {
		return ""
	}
	return client.URI.String()
}

// PID returns the container engine PID, when known.
func (pw *PodmanWatcher) PID() int { return pw.pid }

// Client returns the underlying engine client (engine-specific); in case of
// Podman this is a [context.Context] (sic(k)!) that in turns contains a client.
func (pw *PodmanWatcher) Client() interface{} { return pw.podman }

// Close cleans up and release any engine client resources, if necessary.
func (pw *PodmanWatcher) Close() {
	if pw.podcache != nil {
		pw.podcache.Stop()
	}
	if client, _ := bindings.GetClient(pw.podman); client != nil {
		client.Client.CloseIdleConnections()
	}
}

// List all the currently alive and kicking containers, but do not list any
// containers without any processes.
func (pw *PodmanWatcher) List(svcctx context.Context) ([]*whalewatcher.Container, error) {
	ctx, release := pw.y(svcctx)
	defer release()
	// Scan the currently available containers and take only the alive into
	// further consideration. This is a potentially lengthy operation, as we
	// need to inspect each potential candidate individually due to the way the
	// Docker daemon's API is designed.
	containers, err := containers.List(ctx, nil)
	if err != nil {
		return nil, err // list? what list??
	}
	alives := make([]*whalewatcher.Container, 0, len(containers))
	for _, container := range containers {
		if alive, err := pw.Inspect(svcctx, container.ID); err == nil {
			alives = append(alives, alive)
		} else {
			// silently ignore missing containers that have gone since the list
			// was prepared, but abort on severe problems in order to not keep
			// this running for too long unnecessarily.
			if !engineclient.IsProcesslessContainer(err) && !util.IsNoSuchContainerErr(err) {
				return nil, err
			}
		}
	}
	return alives, nil
}

// Inspect (only) those container details of interest to us, given the name or
// ID of a container.
func (pw *PodmanWatcher) Inspect(svcctx context.Context, nameorid string) (*whalewatcher.Container, error) {
	ctx, release := pw.y(svcctx)
	defer release()

	details, err := containers.Inspect(ctx, nameorid, nil)
	if err != nil {
		return nil, err
	}
	if details.State == nil || details.State.Pid == 0 {
		return nil, engineclient.NewProcesslessContainerError(nameorid, "Podman")
	}
	cntr := &whalewatcher.Container{
		ID:      details.ID,
		Name:    details.Name,
		Labels:  details.Config.Labels,
		PID:     details.State.Pid,
		Project: details.Config.Labels[moby.ComposerProjectLabel],
		Paused:  details.State.Paused,
	}
	if cntr.Labels == nil {
		cntr.Labels = map[string]string{}
	}
	if details.HostConfig != nil && details.HostConfig.Privileged {
		// Just the presence of the "magic" label is sufficient; the label's
		// value doesn't matter.
		cntr.Labels[moby.PrivilegedLabel] = ""
	}
	if details.Pod != "" {
		cntr.Labels[PodIDName] = details.Pod
		cntr.Labels[PodLabelName] = pw.podName(ctx, details.Pod)
	}
	if details.IsInfra {
		cntr.Labels[InfraLabelName] = "" // just mark the presence.
	}
	if pw.packer != nil {
		pw.packer.Pack(cntr, details)
	}
	return cntr, nil
}

// LifecycleEvents streams container engine events, limited just to those events
// in the lifecycle of containers getting born (=alive, as opposed to, say,
// "conceived") and die.
func (pw *PodmanWatcher) LifecycleEvents(svcctx context.Context) (<-chan engineclient.ContainerEvent, <-chan error) {
	ctx, release := pw.y(svcctx)

	cntreventstream := make(chan engineclient.ContainerEvent)
	cntrerrstream := make(chan error, 1)

	go func() {
		defer release()

		// P.o.'d.man client expects us to provide the event channel and on top
		// of this an additional "stop" channel ... because in v3 the client is
		// completely messed up and totally ignores any cancellations to the
		// contexts specified in API calls. *facepalm*
		evs := make(chan entities.Event)
		cancelch := make(chan bool) // close to terminate event watching

		// Please note that the documentation for "podman events"
		// (https://docs.podman.io/en/latest/markdown/podman-events.1.html)
		// doesn't list the "died" event. However, running "podman events
		// --format json" and then starting and terminating a container reveals
		// the correct "died" event status.
		opts := system.EventsOptions{
			Filters: map[string][]string{
				"type": {"container"},
				"status": {
					"start",
					"died",
					"pause",
					"unpause",
				},
			},
		}
		// Yet another P.o.'d.man API design horror: system.Events *blocks*, but
		// also returns an error. This complicates things a lot as we need to
		// kick off a separate go routine but also check for errors. Seriously,
		// this doesn't spark much confidence in the Podman API design.
		apierr := make(chan struct{})
		go func() {
			defer close(cntrerrstream)
			defer close(apierr)
			// system.Events can either return directly with an error, or it can
			// return any time later with an error in the communication with a
			// podman daemon. The latter error might be either genuine or just a
			// result of us closing the cancelch to tell the event listening to
			// stop.
			err := system.Events(ctx, evs, cancelch, &opts)
			// A cancelled context error takes precedence over any event
			// watching error due to not being able to read further from the
			// server event stream.
			if ctxerr := ctx.Err(); ctxerr != nil {
				// Did I mention that the P.o.'d.man implementation is
				// "interesting"? It trips up the go routine leak detection even
				// when taking snapshots before a unit test invoking
				// LifecycleEvents. Warming the API client up before a unit test
				// doesn't help either. Podman really is SNAFU class code, it
				// has code paths that log <nil> errors in normal code paths
				// just due to sheer coding laziness. The only way to not trip
				// up unit test go routine leak detectors is to explicitly close
				// any idle connections here, hoping this will clean up the
				// idling event-related HTTP handling go routines. The only
				// positive thing at the moment is that at least there doesn't
				// seem to be any fd leakages.
				//
				// SCOTTY!!! BEAM ME UP FROM THIS PODMAN CODE BASE!!!
				if conn, _ := bindings.GetClient(pw.podman); conn != nil {
					conn.Client.CloseIdleConnections()
				}
				err = ctxerr
			}
			if err != nil {
				cntrerrstream <- err
			}
		}()
		defer close(cancelch)
		for {
			select {
			case <-apierr:
				return // API error has already been sent down the error channel.
			case <-svcctx.Done():
				// Cover up for the bad job of the v3 client: it completely
				// ignores any cancellation/deadlines of the supplied context.
				// So we need to bail out when we notice that our lifecycle
				// event watching context is "done". "GET CONTEXT DONE".
				// *snicker*
				return // will tell system.Events to cancel.
			case ev := <-evs:
				// This is pretty much boilerplate, as even Podman's own events
				// are Docker-compatible. Red Dan must still be fuming.
				switch ev.Action {
				case "start":
					cntreventstream <- engineclient.ContainerEvent{
						Type:    engineclient.ContainerStarted,
						ID:      ev.Actor.ID,
						Project: ev.Actor.Attributes[moby.ComposerProjectLabel],
					}
				case "died":
					cntreventstream <- engineclient.ContainerEvent{
						Type: engineclient.ContainerExited,
						ID:   ev.Actor.ID,
						// Please note that Podmen v3 and v4 lack support for
						// container labels in "died" events; the default
						// watcher implementation will work around this by
						// looking up the project label if known.
						Project: ev.Actor.Attributes[moby.ComposerProjectLabel],
					}
				case "pause":
					cntreventstream <- engineclient.ContainerEvent{
						Type:    engineclient.ContainerPaused,
						ID:      ev.Actor.ID,
						Project: ev.Actor.Attributes[moby.ComposerProjectLabel],
					}
				case "unpause":
					cntreventstream <- engineclient.ContainerEvent{
						Type:    engineclient.ContainerUnpaused,
						ID:      ev.Actor.ID,
						Project: ev.Actor.Attributes[moby.ComposerProjectLabel],
					}
				}
			}
		}
	}()

	return cntreventstream, cntrerrstream
}

// Returns a podman connection context with the specified context's cancellation
// and deadline mixed into it.
func (pw *PodmanWatcher) y(ctx context.Context) (context.Context, context.CancelFunc) {
	return wye.Mixin(pw.podman, ctx)
}

// podName returns the name of a pod, given only its ID – as is the case with
// the container details which always reference their pod (if any) by ID, never
// by name.
func (pw *PodmanWatcher) podName(ctx context.Context, podid string) string {
	if podname := pw.podcache.Get(podid); podname != nil {
		return podname.Value()
	}
	poddetails, err := pods.Inspect(ctx, podid, &pods.InspectOptions{})
	if err != nil {
		// We don't do negative caching here.
		return ""
	}
	pw.podcache.Set(podid, poddetails.Name, ttlcache.DefaultTTL)
	return poddetails.Name
}
