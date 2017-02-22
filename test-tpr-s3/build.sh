#!/bin/bash

set -e

VERSION=${1:-0.0.28}
DOCKER_REPO="pidah/test-tpr-s3"
DOCKER_IMAGE=${DOCKER_REPO}:${VERSION}

# run tests before build
#echo "Running go test..."
#go test

echo "Building application..."
git tag ${VERSION}
git push origin ${VERSION}
docker run --rm -v "$PWD":/usr/src/myapp -w /usr/src/myapp golang:1.8 bash -c "go get -d && CGO_ENABLED=0 GOOS=linux go build -tags netgo -installsuffix netgo -o test-tpr-s3 -ldflags '-w' -a -v"

echo "Building docker image..."
docker build -t ${DOCKER_IMAGE} .
docker push ${DOCKER_IMAGE}
