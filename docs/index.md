# Kustomizer

Kustomizer is an OSS tool for building Kubernetes continuous delivery workflows.
It offers commands to publish, fetch, customize, validate, and apply Kubernetes configuration.

## Features

### OCI Artifacts

Kustomizer offers a way to distribute Kubernetes configuration using container registries.
It can package Kubernetes manifests in an OCI image and store them in a container registry,
right next to your applications' images.

Kustomizer comes with commands for managing OCI artifacts:

- `kustomizer push artifact oci://<image-url>:<tag> -k [-f] [-p]`
- `kustomizer tag artifact oci://<image-url>:<tag> <new-tag>`
- `kustomizer pull artifact oci://<image-url>:<tag>`
- `kustomizer inspect artifact oci://<image-url>:<tag>`

Kustomizer is compatible with Docker Hub, GHCR, ACR, ECR, GCR, Artifactory,
self-hosted Docker Registry and others. For auth, it uses the credentials from `~/.docker/config.json`.

Assuming you've automated your application's build & push workflow using Docker,
you can extend the automation to do the same for your Kubernetes configuration
that describes how your application gets deployed.

### Resource Inventories

Kustomizer offers a way for grouping Kubernetes resources.
It generates an inventory which keeps track of the set of resources applied together.
The inventory is stored inside the cluster in a `ConfigMap` object and contains metadata
such as the resources provenance and revision.

You specify an inventory name and namespace at apply time, and then you can use Kustomizer to
list, diff, update, and delete inventories:

- `kustomizer apply inventory <name> [--artifact <oci url>] [-f] [-p] -k`
- `kustomizer diff inventory <name> [-a] [-f] [-p] -k`
- `kustomizer get inventories --namespace <namespace>`
- `kustomizer inspect inventory <name> --namespace <namespace>`
- `kustomizer delete inventory <name> --namespace <namespace>`

The Kustomizer garbage collector uses the inventory to keep track of the applied resources
and prunes the Kubernetes objects that were previously applied but are missing from the current revision.

## Comparison with other tools

### vs kubectl

Kustomizer makes use of [k8s.io/cli-runtime](https://pkg.go.dev/k8s.io/cli-runtime)
for loading kubeconfigs and enables users to configure access to Kubernetes clusters
in the same way as with kubectl.

Compared to `kubectl apply -f`, `kustomizer apply -f` does things a little different:

- Validates all resources with dry-run apply, and applies only the ones with changes.
- Applies first custom resource definitions (CRDs) and namespaces, waits for them to register and only then applies the custom resources.
- Waits for the applied resources to be fully reconciled (checks the ready status of replicasets, services, ingresses, and other custom resources).
- Deletes stale objects like ConfigMaps and Secrets generated with Kustomize or other tools.

### vs kustomize

Kustomizer uses the [sigs.k8s.io/kustomize](https://pkg.go.dev/sigs.k8s.io/kustomize/api)
Go packages to patch Kubernetes manifests and is compatible with `kustomize.config.k8s.io/v1beta1` overlays.

Compared to `kustomize build`, `kustomizer build -k` does things a little different:

- Pulls Kubernetes manifests from container registries.
- Reorders the resources according to the provided configuration.
- Allows `kustomization.yaml` to load files from outside their root directory.
- Disallows the usage of Kustomize exec and container-based plugins.
- Extra patches can be specified with `kustomizer build -k ./overlay --patch ./patch1.yaml --patch ./patch2.yaml`.

### vs flux

Kustomizer is akin to Flux's [kustomize-controller](https://github.com/fluxcd/kustomize-controller), and it shares
the same reconcile engine that leverages Kubernetes server-side apply.

Kustomizer can be used as intermediary step when migrating from CI driven deployments
to [Flux](https://fluxcd.io/) and GitOps. If you're running `kubectl apply` in your CI pipelines,
replacing kubectl with kustomizer, would smooth the transition to a continuous delivery system powered by Flux.

At times, Kustomizer servers as a testing bench for experimental features that are proposed to the Flux community.
For example, Kustomizer is the project where features like staged-apply, garbage collection and diffing where first introduced.
