TARGETS := $(shell ls scripts)

VERSION := $(shell grep -w Version version.go | awk '{print $$5}' | sed 's/"//g')
GITHUB_USER := dougoc
MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
CURRENT_DIR := $(notdir $(patsubst %/,%,$(dir $(MAKEFILE_PATH))))
PROJECT := github.com/$(GITHUB_USER)/$(CURRENT_DIR)

.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/latest/dapper-`uname -s`-`uname -m` > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

$(TARGETS): .dapper
	./.dapper $@

trash: .dapper
	./.dapper -m bind trash

trash-keep: .dapper
	./.dapper -m bind trash -k

deps: trash

release:
	git tag | grep -q -w $(VERSION) || git tag $(VERSION)
	ghr --repository $(CURRENT_DIR) \
		--username $(GITHUB_USER) \
		--replace \
		$(VERSION) bin/

.DEFAULT_GOAL := ci

.PHONY: $(TARGETS)
