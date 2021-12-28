# kustomizer

[![report](https://goreportcard.com/badge/github.com/stefanprodan/kustomizer)](https://goreportcard.com/report/github.com/stefanprodan/kustomizer)
[![e2e](https://github.com/stefanprodan/kustomizer/workflows/e2e/badge.svg)](https://github.com/stefanprodan/kustomizer/actions)
[![codecov](https://codecov.io/gh/stefanprodan/kustomizer/branch/main/graph/badge.svg?token=KEU5W1LSZC)](https://codecov.io/gh/stefanprodan/kustomizer)
[![license](https://img.shields.io/github/license/stefanprodan/kustomizer.svg)](https://github.com/stefanprodan/kustomizer/blob/main/LICENSE)
[![release](https://img.shields.io/github/release/stefanprodan/kustomizer/all.svg)](https://github.com/stefanprodan/kustomizer/releases)

Kustomizer is an experimental package manager for distributing Kubernetes configuration as OCI artifacts.
It offers commands to publish, fetch, diff, customize, validate, apply and prune Kubernetes resources.

Kustomizer relies on [server-side apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/)
and requires a Kubernetes cluster **v1.20** or newer.

## Install

The Kustomizer CLI is available as a binary executable for all major platforms,
the binaries can be downloaded from GitHub [releases](https://github.com/stefanprodan/kustomizer/releases).

Install the latest release on macOS or Linux with Homebrew:

```bash
brew install stefanprodan/tap/kustomizer
```

For other installation methods,
see [kustomizer.dev/install](https://kustomizer.dev/install/).

## Get started

To get started with Kustomizer please visit the documentation website at [kustomizer.dev](https://kustomizer.dev/).

## Concepts

### OCI Artifacts

Kustomizer offers a way to distribute Kubernetes configuration using container registries.
It can package Kubernetes manifests in an OCI image and store them in a container registry,
right next to your applications' images.

Kustomizer comes with commands for managing OCI artifacts:

- `kustomizer push artifact oci://<image-url>:<tag> -k [-f] [-p]`
- `kustomizer tag artifact oci://<image-url>:<tag> <new-tag>`
- `kustomizer list artifacts oci://<repo-url> --semver <condition>`
- `kustomizer pull artifact oci://<image-url>:<tag>`
- `kustomizer inspect artifact oci://<image-url>:<tag>`
- `kustomizer diff artifact <oci url> <oci url>`

Kustomizer is compatible with Docker Hub, GHCR, ACR, ECR, GCR, Artifactory,
self-hosted Docker Registry and others. For auth, it uses the credentials from `~/.docker/config.json`.

### Resource Inventories

Kustomizer offers a way for grouping Kubernetes resources.
It generates an inventory which keeps track of the set of resources applied together.
The inventory is stored inside the cluster in a `ConfigMap` object and contains metadata
such as the resources provenance and revision.

The Kustomizer garbage collector uses the inventory to keep track of the applied resources
and prunes the Kubernetes objects that were previously applied but are missing from the current revision.

You specify an inventory name and namespace at apply time, and then you can use Kustomizer to
list, diff, update, and delete inventories:

- `kustomizer apply inventory <name> [--artifact <oci url>] [-f] [-p] -k`
- `kustomizer diff inventory <name> [-a] [-f] [-p] -k`
- `kustomizer get inventories --namespace <namespace>`
- `kustomizer inspect inventory <name> --namespace <namespace>`
- `kustomizer delete inventory <name> --namespace <namespace>`

When applying resources from OCI artifacts, Kustomizer saves the artifacts URL and
the image SHA-2 digest in the inventory. For deterministic and repeatable apply operations,
you could use digests instead of tags.

## Contributing

Kustomizer is [Apache 2.0 licensed](LICENSE) and accepts contributions via GitHub pull requests.
