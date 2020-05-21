#!/bin/bash
set -e

curl -s https://api.github.com/repos/stefanprodan/kustomizer/releases/latest |\
  grep browser_download |\
  grep linux |\
  cut -d '"' -f 4 |\
  xargs curl -sL -o kustomizer.tar.gz

tar xzf ./kustomizer.tar.gz
rm ./kustomizer.tar.gz

mkdir -p $GITHUB_WORKSPACE/bin
mv ./kustomizer $GITHUB_WORKSPACE/bin

echo "::add-path::$GITHUB_WORKSPACE/bin"
echo "::add-path::$RUNNER_WORKSPACE/$(basename $GITHUB_REPOSITORY)/bin"
