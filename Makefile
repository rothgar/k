SOURCE_FILES?=./
TEST_PATTERN?=.
TEST_OPTIONS?=
OS=$(shell uname -s)

export PATH := ./bin:$(PATH)
export LIBRARY_PATH := /usr/lib64

PHONY: test

test:
	go test $(TEST_OPTIONS) -v -failfast -race -coverpkg=./... -covermode=atomic $(SOURCE_FILES) -run $(TEST_PATTERN) -timeout=2m
