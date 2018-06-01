all: depend build test

SHELL := /bin/bash
.DEFAULT_GOAL := all
MODULE_NAME=gpupgrade
AGENT=gpupgrade_agent
CLI=gpupgrade
HUB=gpupgrade_hub
AGENT_PACKAGE=github.com/greenplum-db/gpupgrade/agent
CLI_PACKAGE=github.com/greenplum-db/gpupgrade/cli
HUB_PACKAGE=github.com/greenplum-db/gpupgrade/hub
BIN_DIR=$(shell echo $${UpgradeVersion:-~/go} | awk -F':' '{ print $$1 "/bin"}')

GIT_VERSION := $(shell git describe --tags | perl -pe 's/(.*)-([0-9]*)-(g[0-9a-f]*)/\1+dev.\2.\3/')
UPGRADE_VERSION_STR="-X $(MODULE_NAME)/cli/commanders.UpgradeVersion=$(GIT_VERSION)"

BRANCH := $(shell git for-each-ref --format='%(objectname) %(refname:short)' refs/heads | awk "/^$$(git rev-parse HEAD)/ {print \$$2}")
LINUX_PREFIX := env GOOS=linux GOARCH=amd64
MAC_PREFIX := env GOOS=darwin GOARCH=amd64
LINUX_POSTFIX := .linux.$(BRANCH)
MAC_POSTFIX := .darwin.$(BRANCH)

GOFLAGS := -gcflags="all=-N -l"

dependencies :
		go get -u github.com/golang/protobuf/protoc-gen-go
		go get golang.org/x/tools/cmd/goimports
		go get github.com/golang/lint/golint
		go get github.com/onsi/ginkgo/ginkgo
		go get github.com/alecthomas/gometalinter
		gometalinter --install
		go get github.com/golang/dep/cmd/dep
		dep ensure
# Counterfeiter is not a proper dependency of the app. It is only used occasionally to generate a test class that
# is then checked in.  At the time of that generation, it can be added back to run the dependency list, temporarily.
#		go get github.com/maxbrunsfeld/counterfeiter

depend : dependencies

format :
		goimports -w .
		gofmt -s -w .

lint :
		! gofmt -l agent/ cli/ db/ helpers/ hub/ install/ integrations/ shellparsers/ testutils/ utils/ | read
		gometalinter --config=gometalinter.config -s vendor ./...

unit :
		ginkgo -r -randomizeSuites -randomizeAllSpecs --skipPackage=integrations

integration:
		ginkgo -r -randomizeAllSpecs integrations

test : lint unit integration

coverage :
		@./scripts/show_coverage.sh

sshd_build :
		make -C integrations/sshd

protobuf :
		protoc -I idl/ idl/*.proto --go_out=plugins=grpc:idl
		go get github.com/golang/mock/mockgen
		mockgen -source idl/cli_to_hub.pb.go > mock_idl/cli_to_hub_mock.pb.go
		mockgen -source idl/hub_to_agent.pb.go > mock_idl/hub_to_agent_mock.pb.go

build :
		go build $(GOFLAGS) -o $(BIN_DIR)/$(AGENT) -ldflags $(UPGRADE_VERSION_STR) $(AGENT_PACKAGE)
		go build $(GOFLAGS) -o $(BIN_DIR)/$(CLI) -ldflags $(UPGRADE_VERSION_STR) $(CLI_PACKAGE)
		go build $(GOFLAGS) -o $(BIN_DIR)/$(HUB) -ldflags $(UPGRADE_VERSION_STR) $(HUB_PACKAGE)

build_linux :
		$(LINUX_PREFIX) go build $(GOFLAGS) -o $(BIN_DIR)/$(AGENT)$(LINUX_POSTFIX) -ldflags $(UPGRADE_VERSION_STR) $(AGENT_PACKAGE)
		$(LINUX_PREFIX) go build $(GOFLAGS) -o $(BIN_DIR)/$(CLI)$(LINUX_POSTFIX) -ldflags $(UPGRADE_VERSION_STR) $(CLI_PACKAGE)
		$(LINUX_PREFIX) go build $(GOFLAGS) -o $(BIN_DIR)/$(HUB)$(LINUX_POSTFIX) -ldflags $(UPGRADE_VERSION_STR) $(HUB_PACKAGE)

build_mac:
		$(MAC_PREFIX) go build $(GOFLAGS) -o $(BIN_DIR)/$(AGENT)$(MAC_POSTFIX) -ldflags $(UPGRADE_VERSION_STR) $(AGENT_PACKAGE)
		$(MAC_PREFIX) go build $(GOFLAGS) -o $(BIN_DIR)/$(CLI)$(MAC_POSTFIX) -ldflags $(UPGRADE_VERSION_STR) $(CLI_PACKAGE)
		$(MAC_PREFIX) go build $(GOFLAGS) -o $(BIN_DIR)/$(HUB)$(MAC_POSTFIX) -ldflags $(UPGRADE_VERSION_STR) $(HUB_PACKAGE)

install_agent :
		@psql -t -d template1 -c 'SELECT DISTINCT hostname FROM gp_segment_configuration WHERE content != -1' > /tmp/seg_hosts 2>/dev/null; \
		if [ $$? -eq 0 ]; then \
			gpscp -f /tmp/seg_hosts $(BIN_DIR)/$(AGENT) =:$(GPHOME)/bin/$(AGENT); \
			if [ $$? -eq 0 ]; then \
				echo 'Successfully copied gpupgrade_agent to $(GPHOME) on all segments'; \
			else \
				echo 'Failed to copy gpupgrade_agent to $(GPHOME)'; \
				exit 1; \
			fi; \
		else \
			echo 'Database is not running, please start the database and run this make target again'; \
			exit 1; \
		fi; \
		rm /tmp/seg_hosts

install : build install_agent
		cp -p $(BIN_DIR)/$(CLI) $(GPHOME)/bin/$(CLI)
		cp -p $(BIN_DIR)/$(HUB) $(GPHOME)/bin/$(HUB)

clean:
		# Build artifacts
		rm -f $(BIN_DIR)/$(AGENT)
		rm -f $(BIN_DIR)/$(CLI)
		rm -f $(BIN_DIR)/$(HUB)
		# Test artifacts
		rm -rf /tmp/go-build*
		rm -rf /tmp/gexec_artifacts*
		rm -rf /tmp/ginkgo*
		# Code coverage files
		rm -rf /tmp/cover*
		rm -rf /tmp/unit*
