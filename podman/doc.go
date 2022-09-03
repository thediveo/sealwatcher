/*
Package podman implements the [Podman] [engineclient.EngineClient].

# Podman's Painful Go Bindings

The Podman Go bindings are ... “creative”: they don't define API services on a
Podman “client” but instead use an API endpoint-less(!) client and then pass
this client around as a special value attached to a context(!).

To crank up the pain level further, this context must be the context returned by
[github.com/containers/podman/pkg/bindings.NewConnection] (sic!). Yes, you've
read that right; the function to create a connection actually returns a
[context.Context]. This engine client works around the problem using the [wye]
context mix-in operation.

To add insult to injury, the v3 implementation of Podman completely ignores any
cancellation on its “connection context” or any of its derived contexts.
Instead, the Podman v3 client always uses its own [context.Background]-derived
context. At least v4 fixed this mistake, but as long as, for instance, Debian
and Ubuntu LTS still provide only v3 Podmen, we are stuck with a v3 client (a v3
client can talk to a v4 daemon, but a v4 client can't talk to a v3 daemon).

So Podman really alienates – pun intended – all its API users until the end of
the universe, especially when they need to integrate Podman as yet another
container engine, because so much production code out there gets a context
passed in and passes that (or a derived context) on.

🤦‍♂️

Wait, there's more … Podman's [system.Event] causes left-over go routines that
might or might not get reused, depending on your particular code. In any case
they trip up the [Gomega go routine leak detector]. This engine client
implementation works around this issue by forcing the HTTP client used by a
Podman connection to close all idle connections.

# Podman Incompatibility

Podman v3 and v4 aren't correctly implementing the container “died” event as
they forget to include all labels of the freshly-deceased container, differing
to Docker's “died” event. Luckily, whalewatcher's watcher internal event
processing already handles such a situation as part of [nerdctl] on plain
[containerd].

# Build Notes

Refer to [Podman Getting Started: Building from Scratch] for the long list of
packages to install for your particular build system.

[Podman]: https://podman.io
[wye]: https://github.com/thediveo/wye
[Podman Getting Started: Building from Scratch]: https://podman.io/getting-started/installation#building-from-scratch
[nerdctl]: https://github.com/containerd/nerdctl
[containerd]: https://github.com/containerd/containerd
[Gomega go routine leak detector]: https://onsi.github.io/gomega/#codegleakcode-finding-leaked-goroutines
*/
package podman
