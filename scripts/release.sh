#!/usr/bin/env bash

# get the latest semver tag from git and increment it
# example: v1.0.0 -> v1.0.1
# get one input argument, the type of version to increment
# example: ./scripts/release.sh patch
# get argument if it should release or only local version
# example: ./scripts/release.sh patch release

# check if there is an argument
if [ -z "$1" ]; then
  echo "Please provide the type of version to increment: major, minor, or patch"
  exit 1
fi

# check if there is a second argument
if [ -z "$2" ]; then
  echo "Please provide if it should release or only local version"
  exit 1
fi

# check if the argument is valid
if [ "$1" != "major" ] && [ "$1" != "minor" ] && [ "$1" != "patch" ]; then
  echo "Invalid argument: $1"
  echo "Please provide the type of version to increment: major, minor, or patch"
  exit 1
fi

# check if the second argument is valid
if [ "$2" != "release" ] && [ "$2" != "local" ]; then
  echo "Invalid argument: $2"
  echo "Please provide if it should release or only local version"
  exit 1
fi

# check if there are uncommitted changes if it should release
if [ "$2" == "release" ] && [ -n "$(git status --porcelain)" ]; then
  echo "There are uncommitted changes. Please commit or stash them before creating a new release."
  exit 1
fi

# check if the current branch is up to date with the remote if it should release
if [ "$2" == "release" ] && [ -n "$(git rev-list origin/$(git rev-parse --abbrev-ref HEAD)..HEAD)" ]; then
  echo "The current branch is not up to date with the remote. Please push the changes before creating a new release."
  exit 1
fi

# check if the current branch is main, if it should release
if [ "$2" == "release" ] && [ "$(git rev-parse --abbrev-ref HEAD)" != "main" ]; then
  echo "You can only create a new release from the main branch."
  exit 1
fi

# get the latest tag
latest_tag=$(git describe --tags --abbrev=0)

# get the latest tag's version
latest_version=$(echo "$latest_tag" | cut -c 2-)

# increment the version based on the argument
case $1 in
  major)
    # increment the major version and set minor and patch versions to 0
    new_version=$(echo "$latest_version" | awk -F. -v OFS=. '{$1++; $2=0; $3=0; print}')
    ;;
  minor)
    # increment the minor version and set patch version to 0
    new_version=$(echo "$latest_version" | awk -F. -v OFS=. '{$2++; $3=0; print}')
    ;;
  patch)
    new_version=$(echo "$latest_version" | awk -F. -v OFS=. '{$3++; print}')
    ;;
esac

# create a new tag
new_tag="v$new_version"

# push the new tag
git tag "$new_tag"

# if release check local semvers tags with remote tags and only push the biggest one
if [ "$2" == "release" ]; then
  # get the latest tag from the remote
  latest_remote_tag=$(git ls-remote --tags origin | awk -F/ '{print $3}' | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$" | sort -V | tail -n1)

  # compare the latest tag with the new tag
  if [ "$latest_tag" != "$latest_remote_tag" ]; then
    # push the new tag to the remote
    # git push origin "$new_tag"
    echo "push new tag" "$new_tag"
  fi
fi

# push the new tag to the remote, if it should release
if [ "$2" == "release" ]; then
  git push origin "$new_tag"
fi

# print the new tag
echo "$new_tag"