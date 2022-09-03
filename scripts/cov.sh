#!/bin/bash
set -e

if ! command -v gobadge &>/dev/null; then
    export PATH="$(go env GOPATH)/bin:$PATH"
    if ! command -v gobadge &>/dev/null; then
        go install github.com/AlexBeauchemin/gobadge@latest
    fi
fi

go test -p=1 -v -exec sudo -covermode=atomic -coverprofile=coverage.out ./...
sudo -- $SHELL -c 'chown $SUDO_UID:$SUDO_GID coverage.out'
go tool cover -html=coverage.out -o=coverage.html
go tool cover -func=coverage.out -o=coverage.out
gobadge -filename=coverage.out -green=80 -yellow=50
