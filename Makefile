#Dockerfile vars
govers=1.16.5

#vars
IMAGENAME=openfero
REPO=openfero
APP_VERSION=latest
IMAGEFULLNAME=${REPO}/${IMAGENAME}:${APP_VERSION}

.PHONY: help build push all

help:
		@echo "Makefile arguments:"
		@echo ""
		@echo "govers - Golang Version"
		@echo ""
		@echo "Makefile commands:"
		@echo "build"
		@echo "push"
		@echo "all"

.DEFAULT_GOAL := all

build:
		@lima nerdctl build --pull --build-arg GO_VERS=${govers} -t ${IMAGEFULLNAME} .

push:
		@lima nerdctl push ${IMAGEFULLNAME}

all: build push
