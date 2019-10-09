#!/usr/bin/env bash

set -o errexit
set -o pipefail

if [ -z "$VERSION" ]; then
  echo "Please provide a version with VERSION=x.x.x"
  exit 1;
fi

if [ -z "$IMAGE" ]; then
  IMAGE="nanosapp/vault"
fi

docker build --build-arg "VERSION=$VERSION" -t "$IMAGE:$VERSION" -f image/vault/Dockerfile image/vault/
docker push "$IMAGE:$VERSION"
