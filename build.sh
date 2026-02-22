#!/bin/bash
# Build and push rosecicd container images to the local registry
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REGISTRY="192.168.200.2:5000"
LAST_BUILD_FILE=".last-build"

cd "$SCRIPT_DIR"

# Get current version and git info
VERSION=$(cat VERSION 2>/dev/null | tr -d '\n' || echo "0.1.0")
GIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LAST_HASH=$(cat $LAST_BUILD_FILE 2>/dev/null | tr -d '\n' || echo "")

# Check if we need to bump version
NEEDS_BUMP=false
if [ -n "$(git status --porcelain)" ]; then
    NEEDS_BUMP=true
elif [ "$GIT_HASH" != "$LAST_HASH" ] && [ -n "$LAST_HASH" ]; then
    NEEDS_BUMP=true
fi

if [ "$NEEDS_BUMP" = true ]; then
    IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"
    PATCH=$((PATCH + 1))
    VERSION="${MAJOR}.${MINOR}.${PATCH}"
    echo "$VERSION" > VERSION
    echo "Bumped version to $VERSION"
fi

FULL_VERSION="${VERSION}+${GIT_HASH}"
TAG="v${VERSION}"

# Parse args
BUILD_CONTROLLER=true
BUILD_BUILDER=true
if [ "$1" = "--controller" ]; then
    BUILD_BUILDER=false
elif [ "$1" = "--builder" ]; then
    BUILD_CONTROLLER=false
fi

if [ "$BUILD_BUILDER" = true ]; then
    REPO="rosecicd-builder"
    IMAGE_EDGE="$REGISTRY/$REPO:edge"
    IMAGE_TAG="$REGISTRY/$REPO:$TAG"

    echo "Building $REPO $FULL_VERSION ..."
    podman build --build-arg VERSION="$FULL_VERSION" -f Dockerfile.builder -t "$IMAGE_EDGE" -t "$IMAGE_TAG" .

    echo "Pushing to $REGISTRY ..."
    podman push --tls-verify=false "$IMAGE_EDGE"
    podman push --tls-verify=false "$IMAGE_TAG"

    echo "  $IMAGE_EDGE"
    echo "  $IMAGE_TAG"
fi

if [ "$BUILD_CONTROLLER" = true ]; then
    REPO="rosecicd"
    IMAGE_EDGE="$REGISTRY/$REPO:edge"
    IMAGE_TAG="$REGISTRY/$REPO:$TAG"

    echo "Building $REPO $FULL_VERSION ..."
    podman build --build-arg VERSION="$FULL_VERSION" -f Dockerfile -t "$IMAGE_EDGE" -t "$IMAGE_TAG" .

    echo "Pushing to $REGISTRY ..."
    podman push --tls-verify=false "$IMAGE_EDGE"
    podman push --tls-verify=false "$IMAGE_TAG"

    echo "  $IMAGE_EDGE"
    echo "  $IMAGE_TAG"
fi

# Save build hash
echo "$GIT_HASH" > $LAST_BUILD_FILE

echo ""
echo "=== Build complete ==="
echo "  Version: $FULL_VERSION"
