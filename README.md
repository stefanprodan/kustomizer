# kustomizer

[![e2e](https://github.com/stefanprodan/kustomizer/workflows/e2e/badge.svg)](https://github.com/stefanprodan/kustomizer/actions)
[![license](https://img.shields.io/github/license/stefanprodan/kustomizer.svg)](https://github.com/stefanprodan/kustomizer/blob/master/LICENSE)
[![release](https://img.shields.io/github/release/stefanprodan/kustomizer/all.svg)](https://github.com/stefanprodan/kustomizer/releases)

Kustomizer is command line utility for applying kustomizations on Kubernetes clusters.
Kustomizer garbage collector keeps track of the applied resources and prunes the Kubernetes
objects that were previously applied on the cluster but are missing from the current revision.

