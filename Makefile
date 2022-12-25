# Required for globs to work correctly
SHELL           := /usr/bin/env bash

MODNAME         := $(shell sed -nE 's/^module\s+(.*)\b/\1/p' go.mod)
BINDIR          := bin
BINS            ?= $(shell find cmd -name 'main.go' | cut -d/ -f2)
export GOCACHE  := $(CURDIR)/build

ARCHS           ?= amd64 arm

TAGS            ?=
TESTS           := .
TESTFLAGS       := -race -v
LDFLAGS         :=
ifeq ($(RELEASE),1)
LDFLAGS         += -s -w
else
TAGS            += debug
endif
GOFLAGS         :=

ifneq ($(wildcard $(CURDIR)/.env/.),)
TAGS            += envishere
endif

SRCDIRS         := $(wildcard $(CURDIR)/src $(CURDIR)/internal $(CURDIR)/pkg)
SRC             := $(shell find $(SRCDIRS) -type f -iname '*.go' -print)
CMD_SRC         = $(shell find $(CURDIR)/cmd/$(notdir $@) -type f -iname '*.go' -print)

GIT_COMMIT      := $(shell git rev-parse HEAD 2>/dev/null)
GIT_SHA         := $(shell git rev-parse --short HEAD 2>/dev/null)
GIT_TAG         := $(shell git describe --tags --abbrev=0 --match='v*' --candidates=1 2>/dev/null)
GIT_STATUS      := $(shell test -n "`git status --porcelain 2>/dev/null`" && echo "dirty" || echo "clean")

ifeq ($(GIT_SHA),)
GIT_STATUS      := invalid
endif

ifneq ($(GIT_TAG),)
VERSION         ?= $(GIT_TAG)
else
VERSION         ?= "dev"
endif

VARMODULE        := $(MODNAME)/config
LDFLAGS += -X $(VARMODULE).Version=$(VERSION)
LDFLAGS += -X $(VARMODULE).Commit=$(GIT_COMMIT)
ifeq ($(RELEASE),1)
LDFLAGS += -X $(VARMODULE)._build=release
else
LDFLAGS += -X $(VARMODULE)._build=debug
endif

MULTIARCH:=
ifneq ($(words $(ARCHS)),0)
ifneq ($(words $(ARCHS)),1)
MULTIARCH:=1
endif
endif

ifndef MULTIARCH
OUTPUTS=$(BINS:%=$(BINDIR)/%)
else
OUTPUTS:=$(addprefix $(BINDIR)/,$(foreach arch,$(ARCHS),$(foreach bin,$(BINS),$(arch)/$(bin))))
endif


###################################
# build

.PHONY: all
all: build

ifneq ($(wildcard $(CURDIR)/api/docs/swagger.json),)
.PHONY: swagger
swagger:
	@swag init -g api/swagger.go -o api/docs
endif

.PHONY: proto
proto:
	protoc --go_out=. --go-grpc_out=. \
		--go_opt=paths=source_relative --go-grpc_opt=paths=source_relative \
		internal/ctl/ctl.proto

.PHONY: build
build: $(OUTPUTS)

$(OUTPUTS):%: $(SRC) $(CMD_SRC)
ifdef MULTIARCH
	@$(eval ARCH_ENV:=GOARCH=$(notdir $(patsubst %/,%,$(dir $@))))
else
	@$(eval ARCH_ENV:=)
endif
	@$(eval APPNAME_LDFLAGS:=-X $(VARMODULE).AppName=$(notdir $@))
	@$(ARCH_ENV) go build $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS) $(APPNAME_LDFLAGS)' -o '$@' $(CURDIR)/cmd/$(notdir $@)
	@echo + built: $@


###################################
# tests

.PHONY: test
test: build
#test: test-style
test: test-unit

.PHONY: test-unit
test-unit:
	@go test $(GOFLAGS) -run $(TESTS) ./... $(TESTFLAGS)


###################################
# clean
.PHONY: clean
clean:
	@rm -rf '$(BINDIR)'

.PHONY: mrpopper
mrpopper: clean
	@rm -rf '$(GOCACHE)'


###################################
# build info
.PHONY: info
info:
	 @echo "Version:           $(VERSION)"
	 @echo "Git Tag:           $(GIT_TAG)"
	 @echo "Git Commit:        $(GIT_COMMIT)"
	 @echo "Git Tree Status:   $(GIT_STATUS)"


###################################
# Makefile variables
dump:
	$(foreach v, \
	    $(shell echo "$(filter-out .VARIABLES,$(.VARIABLES))" | tr ' ' '\n' | sort), \
	    $(if $(filter file,$(origin $(v))), $(info $(shell printf "%-20s" "$(v)")= $(value $(v)))) \
	)

dump-expand:
	$(foreach v, \
	    $(shell echo "$(filter-out .VARIABLES,$(.VARIABLES))" | tr ' ' '\n' | sort), \
	    $(if $(filter file,$(origin $(v))), $(info $(shell printf "%-20s" "$(v)")= $($(v)))) \
	)

dump-all:
	$(foreach v, \
	    $(shell echo "$(filter-out .VARIABLES,$(.VARIABLES))" | tr ' ' '\n' | sort), \
	    $(info $(shell printf "%-20s" "$(v)")= $(value $(v))) \
	)
