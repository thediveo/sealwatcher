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
	"os"
	"time"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/thediveo/sealwatcher/test"
	"github.com/thediveo/whalewatcher"
	"github.com/thediveo/whalewatcher/engineclient/moby"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gleak"
	. "github.com/thediveo/fdooze"
	. "github.com/thediveo/success"
	. "github.com/thediveo/whalewatcher/test/matcher"
)

var (
	furiousFuruncle = test.NewContainerDescription{
		Name:   "furious_furuncle",
		Status: test.Running,
		Labels: map[string]string{moby.ComposerProjectLabel: "testproject"},
	}
)

var _ = Describe("podman watcher", func() {

	BeforeEach(func() {
		goodgos := Goroutines()
		goodfds := Filedescriptors()
		DeferCleanup(func() {
			Eventually(Goroutines).ShouldNot(HaveLeaked(goodgos))
			Expect(Filedescriptors()).NotTo(HaveLeakedFds(goodfds))
		})
	})

	It("reports errors", func() {
		Expect(New("unix:///bourish.socket.puppet", nil)).Error().To(HaveOccurred())
	})

	It("watches a container", func(ctx context.Context) {
		if os.Geteuid() != 0 || rootless.IsRootless() /* work around botched podman code base */ {
			Skip("needs root")
		}

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		podconn := Successful(bindings.NewConnection(ctx, "unix:///run/podman/podman.sock"))
		client, _ := bindings.GetClient(podconn)
		defer client.Client.CloseIdleConnections()

		test.RemoveContainer(podconn, furiousFuruncle.Name)

		By("creating a canary container")
		test.NewContainer(podconn, furiousFuruncle)
		defer test.RemoveContainer(podconn, furiousFuruncle.Name)

		pw := Successful(New("unix:///run/podman/podman.sock", nil))
		defer pw.Close()
		go func() {
			defer GinkgoRecover()
			By("starting a watch...")
			Expect(pw.Watch(ctx)).To(MatchError(context.Canceled))
		}()
		By("waiting for the canary to appear")
		Eventually(func() []*whalewatcher.Container {
			proj := pw.Portfolio().Project(furiousFuruncle.Labels[moby.ComposerProjectLabel])
			if proj != nil {
				return proj.Containers()
			}
			return nil
		}).WithTimeout(5 * time.Second).Should(
			ContainElement(HaveName(furiousFuruncle.Name)))

	})

})
