ARCH := $(shell uname -m)
GO_FILES := $(shell find . -name '*.go' -type f)
GENERATOR_FILES := $(shell find cmd/generate -name '*.go' -type f)
SCHEMA_VERSION := $(file < ./schema/version)
SCHEMA_RELEASE_TAG := schema-v$(SCHEMA_VERSION)
GOCACHE ?= $(CURDIR)/.gocache
MDSH ?= mdsh

version: README.md schema/meta.json schema/schema.json schema/meta.unstable.json schema/schema.unstable.json $(GENERATOR_FILES)
	cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .
	gofumpt -w .
	touch $@
	echo $(SCHEMA_VERSION) > $@

schema/meta.json: schema/version
	curl --retry 3 --retry-all-errors -o $@ --fail -L https://github.com/agentclientprotocol/agent-client-protocol/releases/download/$(SCHEMA_RELEASE_TAG)/meta.json

schema/schema.json: schema/version
	curl --retry 3 --retry-all-errors -o $@ --fail -L https://github.com/agentclientprotocol/agent-client-protocol/releases/download/$(SCHEMA_RELEASE_TAG)/schema.json

schema/meta.unstable.json: schema/version
	@set -e; \
		url=https://github.com/agentclientprotocol/agent-client-protocol/releases/download/$(SCHEMA_RELEASE_TAG)/meta.unstable.json; \
		tmp=$@.tmp; \
		status=$$(curl --retry 3 --retry-all-errors -sS -L -o "$$tmp" -w '%{http_code}' "$$url") || { rm -f "$$tmp"; exit 1; }; \
		if [ "$$status" = "200" ]; then \
			mv "$$tmp" $@; \
		elif [ "$$status" = "404" ]; then \
			rm -f "$$tmp"; \
			printf '%s\n' '{"agentMethods":{},"clientMethods":{},"protocolMethods":{}}' > $@; \
		else \
			rm -f "$$tmp"; \
			echo "failed to download $$url (http $$status)" 1>&2; \
			exit 1; \
		fi

schema/schema.unstable.json: schema/version
	@set -e; \
		url=https://github.com/agentclientprotocol/agent-client-protocol/releases/download/$(SCHEMA_RELEASE_TAG)/schema.unstable.json; \
		tmp=$@.tmp; \
		status=$$(curl --retry 3 --retry-all-errors -sS -L -o "$$tmp" -w '%{http_code}' "$$url") || { rm -f "$$tmp"; exit 1; }; \
		if [ "$$status" = "200" ]; then \
			mv "$$tmp" $@; \
		elif [ "$$status" = "404" ]; then \
			rm -f "$$tmp"; \
			printf '%s\n' '{"$$defs":{}}' > $@; \
		else \
			rm -f "$$tmp"; \
			echo "failed to download $$url (http $$status)" 1>&2; \
			exit 1; \
		fi

README.md: schema/version sdk/version
	@command -v $(MDSH) >/dev/null || { echo "mdsh not found; run 'mise install' or install it." 1>&2; exit 1; }
	$(MDSH) --input README.md

.PHONY: fmt
fmt:
	treefmt

# treefmt runs mdsh + mdformat over the markdown, which keeps README.md
# regenerated and in sync (replacing the old mdsh guard). mdsh and mdformat
# disagree on the blank line after an mdsh directive: mdsh strips it, mdformat
# re-adds it. The net result is a no-op (README is a fixpoint of the pair), but
# that intermediate churn trips treefmt's own --fail-on-change. So we format in
# place and let git confirm there's no net drift instead.
.PHONY: check
check:
	treefmt
	git diff --exit-code

.PHONY: test
test: $(GO_FILES)
	GOFLAGS=$(GOFLAGS) GOCACHE=$(GOCACHE) go test ./...
	cd cmd/generate && GOFLAGS=$(GOFLAGS) GOCACHE=$(GOCACHE) go test ./...
	GOFLAGS=$(GOFLAGS) GOCACHE=$(GOCACHE) go build ./example/...

.PHONY: clean
clean:
	rm -f schema/meta.json schema/schema.json schema/meta.unstable.json schema/schema.unstable.json version
	mdsh --clean --input README.md
	touch schema/version # Touching the schema version file ensures that the README.md is regenerated on next make.

.PHONY: release
release:
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required (e.g. 'make release VERSION=0.4.4')" 1>&2; \
		exit 1; \
	fi
	@printf '%s\n' "$(VERSION)" > sdk/version
	$(MAKE) version
	$(MAKE) fmt
	GOCACHE=$(GOCACHE) $(MAKE) test
	@printf '%s\n' "$(VERSION)" | cmp -s - sdk/version || (echo "sdk/version does not match requested SDK release" 1>&2; exit 1)
	@cmp -s schema/version version || (echo "generated schema stamp is stale; rerun 'make version'" 1>&2; exit 1)
	@echo
	@echo "Release candidate for $(VERSION) is ready. Review changes, commit, then tag v$(VERSION)."

.PHONY: update-schema
update-schema:
	@if [ "$(origin SCHEMA_VERSION)" != "command line" ]; then \
		echo "SCHEMA_VERSION is required (e.g. 'make update-schema SCHEMA_VERSION=1.20.0')" 1>&2; \
		exit 1; \
	fi
	@printf '%s\n' "$(SCHEMA_VERSION)" > schema/version
	$(MAKE) version
	$(MAKE) fmt
	@cmp -s schema/version version || (echo "generated schema stamp is stale; rerun 'make version'" 1>&2; exit 1)
	@echo
	@echo "ACP schema $(SCHEMA_VERSION) generated from release schema-v$(SCHEMA_VERSION)."
