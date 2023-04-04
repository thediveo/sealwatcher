.PHONY: help clean coverage pkgsite report test vuln

help: ## list available targets
	@# Shamelessly stolen from Gomega's Makefile
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

clean: ## cleans up build and testing artefacts
	rm -f coverage.html coverage.out coverage.txt

coverage: ## gathers coverage and updates README badge
	@scripts/cov.sh

pkgsite: ## serves Go documentation on port 6060
	@echo "navigate to: http://localhost:6060/github.com/thediveo/sealwatcher/v2"
	@scripts/pkgsite.sh

report: ## run goreportcard on this module
	@scripts/goreportcard.sh

test: ## run unit tests
	@go test -v -exec sudo -p=1 -tags exclude_graphdriver_btrfs,exclude_graphdriver_devicemapper,libdm_no_deferred_remove ./... # -race tripped by podman v3 system.Events

vuln: ## run go vulnerabilities check
	@govulncheck ./...
