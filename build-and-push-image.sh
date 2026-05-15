#!/bin/bash
set -euo pipefail

# Directory of the script file, independent of where it's called from.
DIR="$(cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd)"

# Resolve the current git commit for the build.
COMMIT=$(git -C "$DIR" describe --tags --dirty --always 2>/dev/null || echo "dev")

# Define the container registry, image name, and tag.
CONTAINER_REGISTRY=923662548032.dkr.ecr.us-west-2.amazonaws.com/lightninglabs
IMAGE_NAME=substrate
IMAGE_TAG=${1:-$COMMIT}
DOCKERFILE_PATH="$DIR/Dockerfile"

# Build the Docker image.
echo "Building image $CONTAINER_REGISTRY/$IMAGE_NAME:$IMAGE_TAG (commit: $COMMIT)"
docker buildx build --platform linux/amd64 --load \
    --build-arg COMMIT="$COMMIT" \
    --no-cache -t "$CONTAINER_REGISTRY/$IMAGE_NAME:$IMAGE_TAG" \
    -f "$DOCKERFILE_PATH" "$DIR"

# Push the Docker image to ECR.
docker push "$CONTAINER_REGISTRY/$IMAGE_NAME:$IMAGE_TAG"
