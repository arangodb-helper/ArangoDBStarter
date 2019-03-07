PROJECT := arangodb
ifndef SCRIPTDIR
	SCRIPTDIR := $(shell pwd)
endif
ROOTDIR := $(shell cd $(SCRIPTDIR) && pwd)
VERSION := $(shell cat $(ROOTDIR)/VERSION)
VERSION_MAJOR_MINOR_PATCH := $(shell echo $(VERSION) | cut -f 1 -d '+')
VERSION_MAJOR_MINOR := $(shell echo $(VERSION_MAJOR_MINOR_PATCH) | cut -f 1,2 -d '.')
VERSION_MAJOR := $(shell echo $(VERSION_MAJOR_MINOR) | cut -f 1 -d '.')
COMMIT := $(shell git rev-parse --short HEAD)
MAKEFILE := $(ROOTDIR)/Makefile

ifndef NODOCKER
	DOCKERCLI := $(shell which docker)
	GOBUILDLINKTARGET := ../../../..
else
	DOCKERCLI := 
	GOBUILDLINKTARGET := $(ROOTDIR)
endif

ifndef BUILDDIR
	BUILDDIR := $(ROOTDIR)
endif
GOBUILDDIR := $(BUILDDIR)/.gobuild
SRCDIR := $(SCRIPTDIR)
CACHEVOL := $(PROJECT)-gocache
BINDIR := $(BUILDDIR)/bin

ORGPATH := github.com/arangodb-helper
ORGDIR := $(GOBUILDDIR)/src/$(ORGPATH)
REPONAME := $(PROJECT)
REPODIR := $(ORGDIR)/$(REPONAME)
REPOPATH := $(ORGPATH)/$(REPONAME)

GOPATH := $(GOBUILDDIR)
GOVERSION := 1.10.3-alpine
GOHOSTOS := $(shell go env GOHOSTOS)

ifndef GOOS
	GOOS := linux
endif
ifndef GOARCH
	GOARCH := amd64
endif
ifeq ("$(GOOS)", "windows")
	GOEXE := .exe
endif

ifndef ARANGODB
	ARANGODB := arangodb/arangodb:latest
endif

ifndef DOCKERTAG 
	DOCKERTAG := dev
endif
DOCKERIMAGENAME := $(DOCKERNAMESPACE)/arangodb-starter
DOCKERIMAGE := $(DOCKERIMAGENAME):$(DOCKERTAG)

ifdef TRAVIS
	IP := $(shell hostname -I | cut -d ' ' -f 1)
	echo Using IP=$(IP)
endif

TEST_TIMEOUT := 25m

BINNAME := arangodb$(GOEXE)
BIN := $(BINDIR)/$(GOOS)/$(GOARCH)/$(BINNAME)
RELEASE := $(GOBUILDDIR)/bin/release 
GHRELEASE := $(GOBUILDDIR)/bin/github-release 
MANIFESTTOOL := $(GOBUILDDIR)/bin/manifest-tool$(shell go env GOEXE)

# Magical rubbish to teach make what commas and spaces are.
EMPTY :=
SPACE := $(EMPTY) $(EMPTY)
COMMA := $(EMPTY),$(EMPTY)

ARCHS:=amd64 arm64
PLATFORMS:=$(subst $(SPACE),$(COMMA),$(foreach arch,$(ARCHS),linux/$(arch)))

SOURCES := $(shell find $(SRCDIR) -name '*.go' -not -path './test/*')

.PHONY: all clean deps docker build build-local

all: build

clean:
	rm -Rf $(BIN) $(BINDIR) $(GOBUILDDIR) $(ROOTDIR)/arangodb

local:
ifneq ("$(DOCKERCLI)", "")
	@${MAKE} -f $(MAKEFILE) -B GOOS=$(shell go env GOHOSTOS) GOARCH=$(shell go env GOHOSTARCH) build-local
else
	@${MAKE} -f $(MAKEFILE) deps
	GOPATH=$(GOBUILDDIR) go build -o $(BUILDDIR)/arangodb $(REPOPATH)
endif

build: check-vars $(BIN)

.PHONY: check-vars
check-vars:
ifndef DOCKERNAMESPACE
	@echo "DOCKERNAMESPACE must be set"
	@exit 1
endif
	@echo "Using docker namespace: $(DOCKERNAMESPACE)"

.PHONY: check-manifest-vars
check-manifest-vars:
ifndef MANIFESTAUTH
ifeq ("$(GOHOSTOS)", "darwin")
		@echo "MANIFESTAUTH must be set on macOS"
		@exit 1
endif
endif

build-local: build 
	@ln -sf $(BIN) $(ROOTDIR)/arangodb

binaries: $(GHRELEASE)
	@${MAKE} -f $(MAKEFILE) -B GOOS=linux GOARCH=amd64 build
	@${MAKE} -f $(MAKEFILE) -B GOOS=linux GOARCH=arm64 build
	@${MAKE} -f $(MAKEFILE) -B GOOS=darwin GOARCH=amd64 build
	@${MAKE} -f $(MAKEFILE) -B GOOS=windows GOARCH=amd64 build

deps:
	@${MAKE} -f $(MAKEFILE) -B SCRIPTDIR=$(SCRIPTDIR) BUILDDIR=$(BUILDDIR) -s $(GOBUILDDIR)

$(GOBUILDDIR):
	@mkdir -p $(ORGDIR)
	@mkdir -p $(GOBUILDDIR)/src/golang.org
	@mkdir -p $(GOBUILDDIR)/src/github.com/arangodb
	@rm -f $(REPODIR) && ln -s $(GOBUILDLINKTARGET) $(REPODIR)
	@rm -f $(GOBUILDDIR)/src/github.com/aktau && ln -s ../../../deps/github.com/aktau $(GOBUILDDIR)/src/github.com/aktau
	@rm -f $(GOBUILDDIR)/src/github.com/arangodb/go-driver && ln -s ../../../../deps/github.com/arangodb/go-driver $(GOBUILDDIR)/src/github.com/arangodb/go-driver
	@rm -f $(GOBUILDDIR)/src/github.com/arangodb/go-velocypack && ln -s ../../../../deps/github.com/arangodb/go-velocypack $(GOBUILDDIR)/src/github.com/arangodb/go-velocypack
	@rm -f $(GOBUILDDIR)/src/github.com/arangodb-helper/go-certificates && ln -s ../../../../deps/github.com/arangodb-helper/go-certificates $(GOBUILDDIR)/src/github.com/arangodb-helper/go-certificates
	@rm -f $(GOBUILDDIR)/src/github.com/cenkalti && ln -s ../../../deps/github.com/cenkalti $(GOBUILDDIR)/src/github.com/cenkalti
	@rm -f $(GOBUILDDIR)/src/github.com/coreos && ln -s ../../../deps/github.com/coreos $(GOBUILDDIR)/src/github.com/coreos
	@rm -f $(GOBUILDDIR)/src/github.com/dchest && ln -s ../../../deps/github.com/dchest $(GOBUILDDIR)/src/github.com/dchest
	@rm -f $(GOBUILDDIR)/src/github.com/dgrijalva && ln -s ../../../deps/github.com/dgrijalva $(GOBUILDDIR)/src/github.com/dgrijalva
	@rm -f $(GOBUILDDIR)/src/github.com/docker && ln -s ../../../deps/github.com/docker $(GOBUILDDIR)/src/github.com/docker
	@rm -f $(GOBUILDDIR)/src/github.com/dustin && ln -s ../../../deps/github.com/dustin $(GOBUILDDIR)/src/github.com/dustin
	@rm -f $(GOBUILDDIR)/src/github.com/estesp && ln -s ../../../deps/github.com/estesp $(GOBUILDDIR)/src/github.com/estesp
	@rm -f $(GOBUILDDIR)/src/github.com/fatih && ln -s ../../../deps/github.com/fatih $(GOBUILDDIR)/src/github.com/fatih
	@rm -f $(GOBUILDDIR)/src/github.com/fsouza && ln -s ../../../deps/github.com/fsouza $(GOBUILDDIR)/src/github.com/fsouza
	@rm -f $(GOBUILDDIR)/src/github.com/hashicorp && ln -s ../../../deps/github.com/hashicorp $(GOBUILDDIR)/src/github.com/hashicorp
	@rm -f $(GOBUILDDIR)/src/github.com/inconshreveable && ln -s ../../../deps/github.com/inconshreveable $(GOBUILDDIR)/src/github.com/inconshreveable
	@rm -f $(GOBUILDDIR)/src/github.com/mitchellh && ln -s ../../../deps/github.com/mitchellh $(GOBUILDDIR)/src/github.com/mitchellh
	@rm -f $(GOBUILDDIR)/src/github.com/Microsoft && ln -s ../../../deps/github.com/Microsoft $(GOBUILDDIR)/src/github.com/Microsoft
	@rm -f $(GOBUILDDIR)/src/github.com/kballard && ln -s ../../../deps/github.com/kballard $(GOBUILDDIR)/src/github.com/kballard
	@rm -f $(GOBUILDDIR)/src/github.com/pavel-v-chernykh && ln -s ../../../deps/github.com/pavel-v-chernykh $(GOBUILDDIR)/src/github.com/pavel-v-chernykh
	@rm -f $(GOBUILDDIR)/src/github.com/pkg && ln -s ../../../deps/github.com/pkg $(GOBUILDDIR)/src/github.com/pkg
	@rm -f $(GOBUILDDIR)/src/github.com/rs && ln -s ../../../deps/github.com/rs $(GOBUILDDIR)/src/github.com/rs
	@rm -f $(GOBUILDDIR)/src/github.com/shavac && ln -s ../../../deps/github.com/shavac $(GOBUILDDIR)/src/github.com/shavac
	@rm -f $(GOBUILDDIR)/src/github.com/spf13 && ln -s ../../../deps/github.com/spf13 $(GOBUILDDIR)/src/github.com/spf13
	@rm -f $(GOBUILDDIR)/src/github.com/ryanuber && ln -s ../../../deps/github.com/ryanuber $(GOBUILDDIR)/src/github.com/ryanuber
	@rm -f $(GOBUILDDIR)/src/github.com/voxelbrain && ln -s ../../../deps/github.com/voxelbrain $(GOBUILDDIR)/src/github.com/voxelbrain
	@rm -f $(GOBUILDDIR)/src/golang.org/x && ln -s ../../../deps/golang.org/x $(GOBUILDDIR)/src/golang.org/x
	@GOPATH=$(GOBUILDDIR) go get github.com/arangodb/go-upgrade-rules

.PHONY: $(CACHEVOL)
$(CACHEVOL):
	@docker volume create $(CACHEVOL)

$(BIN): $(GOBUILDDIR) $(SOURCES) $(CACHEVOL)
	@mkdir -p $(BINDIR)
	docker run \
		--rm \
		-v $(SRCDIR):/usr/code \
		-v $(CACHEVOL):/usr/gocache \
		-e GOCACHE=/usr/gocache \
		-e GOPATH=/usr/code/.gobuild \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-e CGO_ENABLED=0 \
		-w /usr/code/ \
		golang:$(GOVERSION) \
		go build -installsuffix netgo -tags netgo -ldflags "-X main.projectVersion=$(VERSION) -X main.projectBuild=$(COMMIT)" -o /usr/code/bin/$(GOOS)/$(GOARCH)/$(BINNAME) $(REPOPATH)

docker: check-manifest-vars $(MANIFESTTOOL)
	for arch in $(ARCHS); do \
		$(MAKE) -f $(MAKEFILE) -B GOOS=linux GOARCH=$$arch build ;\
		docker build --build-arg=GOARCH=$$arch -t $(DOCKERIMAGE)-$$arch . ;\
		docker push $(DOCKERIMAGE)-$$arch ;\
	done
	$(MANIFESTTOOL) $(MANIFESTAUTH) push from-args \
    	--platforms $(PLATFORMS) \
    	--template $(DOCKERIMAGE)-ARCH \
    	--target $(DOCKERIMAGE)

docker-push: docker
	docker push $(DOCKERIMAGE)

docker-push-version: docker
	docker tag $(DOCKERIMAGE) $(DOCKERIMAGENAME):$(VERSION)
	docker tag $(DOCKERIMAGE) $(DOCKERIMAGENAME):$(VERSION_MAJOR_MINOR)
	docker tag $(DOCKERIMAGE) $(DOCKERIMAGENAME):$(VERSION_MAJOR)
	docker tag $(DOCKERIMAGE) $(DOCKERIMAGENAME):latest
	docker push $(DOCKERIMAGENAME):$(VERSION)
	docker push $(DOCKERIMAGENAME):$(VERSION_MAJOR_MINOR)
	docker push $(DOCKERIMAGENAME):$(VERSION_MAJOR)
	docker push $(DOCKERIMAGE)

$(RELEASE): $(GOBUILDDIR) $(SOURCES) $(GHRELEASE)
	GOPATH=$(GOBUILDDIR) go build -o $(RELEASE) $(REPOPATH)/tools/release

$(GHRELEASE): $(GOBUILDDIR) 
	GOPATH=$(GOBUILDDIR) go build -o $(GHRELEASE) github.com/aktau/github-release

$(MANIFESTTOOL): $(GOBUILDDIR)
	GOPATH=$(GOBUILDDIR) go build -o $(MANIFESTTOOL) github.com/estesp/manifest-tool

release-patch: $(RELEASE)
	GOPATH=$(GOBUILDDIR) $(RELEASE) -type=patch 

release-minor: $(RELEASE)
	GOPATH=$(GOBUILDDIR) $(RELEASE) -type=minor

release-major: $(RELEASE)
	GOPATH=$(GOBUILDDIR) $(RELEASE) -type=major 

TESTCONTAINER := arangodb-starter-test

test-images:
	docker pull $(ARANGODB)
	docker build --build-arg "from=$(ARANGODB)" -t arangodb-golang -f test/Dockerfile-arangodb-golang .

# Run all integration tests
run-tests: run-tests-local-process run-tests-docker

run-tests-local-process: build test-images
	@-docker rm -f -v $(TESTCONTAINER) &> /dev/null
	docker run \
		--rm \
		--name=$(TESTCONTAINER) \
		-v $(ROOTDIR):/usr/code \
		-e CGO_ENABLED=0 \
		-e GOCACHE=off \
		-e GOPATH=/usr/code/.gobuild \
		-e DATA_DIR=/tmp \
		-e STARTER=/usr/code/bin/linux/amd64/arangodb \
		-e TEST_MODES=localprocess \
		-e STARTER_MODES=$(STARTER_MODES) \
		-e ENTERPRISE=$(ENTERPRISE) \
		-e TESTOPTIONS=$(TESTOPTIONS) \
		-e DEBUG_CLUSTER=$(DEBUG_CLUSTER) \
		-w /usr/code/ \
		arangodb-golang \
		go test -timeout $(TEST_TIMEOUT) $(TESTOPTIONS) -v $(REPOPATH)/test

run-tests-docker: docker
ifdef TRAVIS
	docker pull $(ARANGODB)
endif
	mkdir -p $(GOBUILDDIR)/tmp
	GOCACHE=off GOPATH=$(GOBUILDDIR) TMPDIR=$(GOBUILDDIR)/tmp TEST_MODES=docker STARTER_MODES=$(STARTER_MODES) ENTERPRISE=$(ENTERPRISE) IP=$(IP) ARANGODB=$(ARANGODB) go test -timeout $(TEST_TIMEOUT) $(TESTOPTIONS) -v $(REPOPATH)/test

# Run all integration tests on the local system
run-tests-local: local
	GOCACHE=off GOPATH=$(GOBUILDDIR) TEST_MODES="localprocess,docker" STARTER_MODES=$(STARTER_MODES) STARTER=$(ROOTDIR)/arangodb go test -timeout $(TEST_TIMEOUT) $(TESTOPTIONS) -v $(REPOPATH)/test
