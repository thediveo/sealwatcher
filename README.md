<!-- markdownlint-disable-next-line MD022 -->
# Sealwatcher
<img align="right" width="200" alt="sealwatcher" src="docs/_images/sealwatcher.png">

[![PkgGoDev](https://pkg.go.dev/badge/github.com/thediveo/sealwatcher)](https://pkg.go.dev/github.com/thediveo/sealwatcher)
[![GitHub](https://img.shields.io/github/license/thediveo/sealwatcher)](https://img.shields.io/github/license/thediveo/sealwatcher)
![build and test](https://github.com/thediveo/sealwatcher/workflows/build%20and%20test/badge.svg?branch=master)
![goroutines](https://img.shields.io/badge/go%20routines-not%20leaking-success)
![Coverage](https://img.shields.io/badge/Coverage-93.9%25-brightgreen)
[![Go Report Card](https://goreportcard.com/badge/github.com/thediveo/sealwatcher)](https://goreportcard.com/report/github.com/thediveo/sealwatcher)

`sealwatcher` adds [Podman](https://podman.io) support to
[@thediveo/whalewatcher](https://github.com/thediveo/whalewatcher) in order to
track the list of containers (name, PID, project, pod) without constant polling
and without the hassle of event "binge processing".

**Note:** `sealwatcher/v2` requires podman 4+ as it uses the podman API v4
client – and version compatibility of podman is very limited and very weak when
compared to Docker API compatibility.

**Note:** because building the Podman REST API client requires a considerable
amount of C libraries as well as header files to be installed in the build
system, `sealwatcher` isn't an integral part of the `whalewatcher` module so
far. If at some future point the Podman project improves the situation so that
Podman REST API clients can be built without the huge installation overhead,
then `sealwatcher` might be finally integrated into `whalewatcher`.

## Installation

First, install the non-Go stuff the Podman module insists of having available,
even if it is totally unnecessary for a REST API client. The following is a
massively stripped-down version of the Debian/Ubuntu package list from [Podman's
"Building from
scratch"](https://podman.io/getting-started/installation#building-from-scratch)
instructions:

```bash
sudo apt-get -y install build-essential pkg-config libbtrfs-dev libgpgme-dev
```

...then you can `go get` v2 of the `sealwatcher` module.

```bash
go get github.com/thediveo/sealwatcher/v2@latest
```

Finally, when building your application using `sealwatcher` directly or
indirectly, use these build tags:

```
-tags exclude_graphdriver_btrfs,exclude_graphdriver_devicemapper,libdm_no_deferred_remove
```

## Supported Go Versions

`sealwatcher` supports versions of Go that are noted by the [Go release
policy](https://golang.org/doc/devel/release.html#policy), that is, _N_ and
_N_-1 major versions.

## Miscellaneous

- to view the package documentation _locally_:
  - either: `make pkgsite`,
  - or, in VSCode (using the VSCode-integrated simple browser): “Tasks: Run
    Task” ⇢ “View Go module documentation”.
- `make` shows the available make targets.

## Hacking It

This project comes with comprehensive unit tests, even covering goroutine and
file descriptor leak checks:

* goroutine leak checking courtesy of Gomega's
  [`gleak`](https://onsi.github.io/gomega/#codegleakcode-finding-leaked-goroutines)
  package.

* file descriptor leak checking courtesy of the
  [@thediveo/fdooze](https://github.com/thediveo/fdooze) module.

> **Note:** do **not run parallel tests** for multiple packages. `make test`
ensures to run all package tests always sequentially, but in case you run `go
test` yourself, please don't forget `-p 1` when testing multiple packages in
one, _erm_, go.

## Copyright and License

Copyright 2022-23 Harald Albrecht, licensed under the Apache License, Version
2.0.
