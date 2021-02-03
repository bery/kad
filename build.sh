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

# sign image
podman image sign --sign-by tom+imagesign@6shore.net -d /var/lib/atomic/sigstore "docker://${R_TAG}"

# sync staging signatures with sigstore
scp -vr /var/lib/atomic/sigstore/tomkukral/ lemur.6shore.net:/mnt/storage/sigstore/

# try to pull image
podman pull --log-level debug "${R_TAG}"
