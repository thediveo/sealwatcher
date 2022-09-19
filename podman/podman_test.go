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
	"time"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/containers"
	"github.com/containers/podman/v3/pkg/bindings/pods"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/specgen"
	"github.com/thediveo/sealwatcher/test"
	"github.com/thediveo/whalewatcher"
	"github.com/thediveo/whalewatcher/engineclient"
	"github.com/thediveo/whalewatcher/engineclient/moby"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gleak"
	. "github.com/thediveo/fdooze"
	. "github.com/thediveo/whalewatcher/test/matcher"
)

type packer struct{}

func (p *packer) Pack(container *whalewatcher.Container, inspection interface{}) {
	Expect(container).NotTo(BeNil())
	Expect(inspection).NotTo(BeNil())
	var details *define.InspectContainerData
	Expect(inspection).To(BeAssignableToTypeOf(details))
	details = inspection.(*define.InspectContainerData)
	container.Rucksack = &details
}

var (
	furiousFuruncle = test.NewContainerDescription{
		Name:   "furious_furuncle",
		Status: test.Running,
		Labels: map[string]string{moby.ComposerProjectLabel: "testproject"},
	}

	deadDummy = test.NewContainerDescription{
		Name:   "dead_dummy",
		Status: test.Created,
	}

	madMay = test.NewContainerDescription{
		Name:   "mad_mary",
		Status: test.Running,
		Labels: map[string]string{moby.ComposerProjectLabel: "testproject"},
	}
)

var _ = Describe("podman engineclient", Ordered, func() {

	var podconn context.Context /* facepalm */
	var pw *PodmanWatcher

	BeforeAll(func() {
		var err error
		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(func() {
			cancel()
		})

		podconn, err = bindings.NewConnection(ctx, "unix:///var/run/podman/podman.sock")
		Expect(err).NotTo(HaveOccurred())

		test.RemoveContainer(podconn, furiousFuruncle.Name)
		test.RemoveContainer(podconn, madMay.Name)
		test.RemoveContainer(podconn, deadDummy.Name)

		pw = NewPodmanWatcher(podconn, WithPID(12345))
		DeferCleanup(func() {
			pw.Close()
		})

		test.NewContainer(podconn, furiousFuruncle)
		test.NewContainer(podconn, deadDummy)
		DeferCleanup(func() {
			test.RemoveContainer(podconn, furiousFuruncle.Name)
			test.RemoveContainer(podconn, madMay.Name)
			test.RemoveContainer(podconn, deadDummy.Name)
		})
	})

	BeforeEach(func() {
		goodgos := Goroutines()
		goodfds := Filedescriptors()
		DeferCleanup(func() {
			// The p.o.'d.man client *is* nasty.
			conn, _ := bindings.GetClient(podconn)
			conn.Client.CloseIdleConnections()

			Eventually(Goroutines).WithTimeout(2 * time.Second).ShouldNot(HaveLeaked(goodgos))
			Expect(Filedescriptors()).NotTo(HaveLeakedFds(goodfds))
		})
	})

	It("remembers the PID", func() {
		Expect(pw.PID()).To(Equal(12345))
	})

	It("has engine type ID and API path", func() {
		Expect(pw.Type()).To(Equal(Type))
		Expect(pw.API()).NotTo(BeEmpty())
	})

	It("has an ID and version", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		Expect(pw.ID(ctx)).ToNot(BeEmpty())
		Expect(pw.Version(ctx)).To(MatchRegexp(`\d+.\d+.\d+`))
	})

	It("sets a rucksack packer", func() {
		p := packer{}
		pw := NewPodmanWatcher(podconn, WithRucksackPacker(&p))
		Expect(pw).NotTo(BeNil())
		defer pw.Close()
		Expect(pw.packer).To(BeIdenticalTo(&p))
	})

	It("inspects a furuncle", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		defer func() { pw.packer = nil }()
		pw.packer = &packer{}

		cntr, err := pw.Inspect(ctx, furiousFuruncle.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(cntr).To(HaveName(furiousFuruncle.Name))
		Expect(cntr).To(HaveField("ID", Not(BeEmpty())))
		Expect(cntr).To(HaveProject(furiousFuruncle.Labels[moby.ComposerProjectLabel]))
		Expect(cntr.Paused).To(BeFalse())
		Expect(cntr.Labels).NotTo(HaveKey(moby.PrivilegedLabel))
		Expect(cntr.Rucksack).NotTo(BeNil())
	})

	It("can't inspect a dead_dummy", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		Expect(pw.Inspect(ctx, deadDummy.Name)).Error().To(HaveOccurred())
	})

	It("returns an error when trying to inspect a non-existing container", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		Expect(pw.Inspect(ctx, "totally-non-existing-container-name")).Error().
			To(HaveField("Because", "no such container"))
	})

	It("inspects and lists a furuncle", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		Expect(pw.Inspect(ctx, furiousFuruncle.Name)).Error().NotTo(HaveOccurred())
		Expect(pw.List(ctx)).To(ContainElement(HaveName(furiousFuruncle.Name)))
	})

	It("watches containers come and go", func() {
		ctx, cancel := context.WithCancel(context.Background())

		evs, errs := pw.LifecycleEvents(ctx)
		Expect(evs).NotTo(BeNil())
		Expect(errs).NotTo(BeNil())

		Consistently(evs).ShouldNot(Receive())
		Consistently(errs).ShouldNot(Receive())

		By("adding a new container")
		id := test.NewContainer(podconn, madMay, test.AsPrivileged())
		Eventually(evs).WithTimeout(5 * time.Second).Should(Receive(And(
			HaveID(id),
			HaveEventType(engineclient.ContainerStarted),
			HaveProject(madMay.Labels[moby.ComposerProjectLabel]),
		)))
		Expect(pw.Inspect(ctx, id)).To(
			HaveField("Labels", HaveKey(moby.PrivilegedLabel)))

		By("pausing the container")
		containers.Pause(podconn, madMay.Name, nil)
		Eventually(evs).WithTimeout(5 * time.Second).Should(Receive(And(
			HaveID(id),
			HaveEventType(engineclient.ContainerPaused),
			HaveProject(madMay.Labels[moby.ComposerProjectLabel]),
		)))

		By("unpausing the container")
		containers.Unpause(podconn, madMay.Name, nil)
		Eventually(evs).WithTimeout(5 * time.Second).Should(Receive(And(
			HaveID(id),
			HaveEventType(engineclient.ContainerUnpaused),
			HaveProject(madMay.Labels[moby.ComposerProjectLabel]),
		)))

		By("removing the container")
		test.RemoveContainer(podconn, madMay.Name)
		Eventually(evs).WithTimeout(5 * time.Second).Should(Receive(And(
			HaveID(id),
			HaveEventType(engineclient.ContainerExited),
			// HaveProject(madMay.Labels[moby.ComposerProjectLabel]), // due to podman v3/v4 incompatibility
		)))

		By("cancelling the lifecycle event stream context")
		cancel()
		Eventually(errs).Should(Receive(Equal(ctx.Err())))

		By("done")
	})

	It("returns an empty name for a non-existing pod ID", func() {
		Expect(pw.podName(podconn, "---podname-not-for-sale---")).To(BeEmpty())
	})

	It("determines pod names of containers", func() {
		const podname = "dizzy_lizzy"

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		By("creating a pod")
		pod, err := pods.CreatePodFromSpec(podconn, &entities.PodSpec{
			PodSpecGen: specgen.PodSpecGenerator{
				PodBasicConfig: specgen.PodBasicConfig{
					Name: podname,
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			force := true
			pods.Remove(podconn, podname, &pods.RemoveOptions{Force: &force})
		}()

		By("creating a container in the pod")
		id := test.NewContainer(podconn, madMay, test.OfPod(podname))

		By("watching them to appear")
		Eventually(func() []*whalewatcher.Container {
			cntrs, _ := pw.List(ctx)
			return cntrs
		}).WithTimeout(5 * time.Second).Should(
			ContainElements(
				And(
					HaveID(id),
					HaveField("Labels", And(
						HaveKeyWithValue(PodLabelName, podname),
						HaveKeyWithValue(PodIDName, pod.Id))),
				),
				And(
					HaveField("Labels", And(
						HaveKey(InfraLabelName),
						HaveKeyWithValue(PodLabelName, podname),
						HaveKeyWithValue(PodIDName, pod.Id),
					)),
				),
			))

		By("done")
	})

})
