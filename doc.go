/*
Package sealwatcher provides a container [watcher.Watcher] for [Podman] engines.

# Usage

	import "github.com/thediveo/sealwatcher"
	watcher := sealwatcher.NewWatcher("", nil) // with default backoff

The watcher constructor accepts options, with currently the only option being
specifying a container engine's PID. The PID information then can be used
downstream in tools like [lxkns] to translate container PIDs between different
PID namespaces.

# Notes

This package adds the following Podman-specific "annotation" labels to the
discovered containers:

  - io.github.thediveo/podman/podname ([PodLabelName]) – if present, the name of
    the corresponding pod the container belongs to.
  - io.github.thediveo/podman/infra ([InfraLabelName]) – just the presence of
    this label marks a container as an “infrastructure” container, its value
    doesn't matter and must not be relied upon.

[Podman]: https://podman.io
[lxkns]: https://github.com/thediveo/lxkns
*/
package sealwatcher
