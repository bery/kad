#!/bin/bash -x

R_BRANCH="${BRANCH:-$(git rev-parse --abbrev-ref HEAD)}"
R_COMMIT="${COMMIT:-$(git rev-parse HEAD)}"
R_TAG="${TAG:-tomkukral/kad:latest}"


# build container using podman
podman build \
	--format docker \
	--label branch="${R_BRANCH}" \
	--label commit="${R_COMMIT}" \
	--tag "${R_TAG}" .

# parse digest
DIGEST="$(podman inspect tomkukral/kad | jq -r '.[0].Digest')"
echo "Image digest is ${DIGEST}"
