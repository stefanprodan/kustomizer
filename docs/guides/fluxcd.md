# Continuous deployment with Kustomizer and Flux

This guide shows you how to continuously deploy applications to Kubernetes
clusters with Flux using OCI artifacts produced by Kustomizer.

This guide offers a better alternative to deploying applications
with GitHub CI (as showcased in the [Deploy from Git](deploy-from-git.md) guide).
Instead of connecting to each Kubernetes cluster from GitHub Actions,
we'll use CI for pushing OCI artifacts to a container registry, and from
there, the Kubernetes clusters (running Flux) will drive the app deployment
themselves. One major advantage to this approach, is that you no longer have
to deal with securing the access from CI to your production systems.

## Before you begin

- Install Kustomizer and the Flux CLI.
- Have a Kubernetes cluster version 1.20 or newer.
- Have a GitHub account.

!!! info "Install with Homebrew"

    ```
    brew install stefanprodan/tap/kustomizer fluxcd/tap/flux
    ```

## Login to GitHub Container Registry

Export you GitHub username:

```shell
export GITHUB_USER="YOUR-GITHUB-USERNAME"
```

Generate a [personal access token (PAT)](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
with read and write access to GitHub Container Registry.

```shell
export GITHUB_TOKEN="YOUR-GITHUB-PAT"
```

Use the token to sign in to the container registry service at ghcr.io:

```console
$ echo $GITHUB_TOKEN | docker login ghcr.io -u ${GITHUB_USER} --password-stdin
> Login Succeeded
```

!!! info "Other container registries"

    Besides GHCR, both Kustomizer and Flux are compatible with Docker Hub, ACR, ECR, GCR,
    self-hosted Docker Registry v2 and any other registry that conforms
    to the [Open Container Initiative](https://opencontainers.org/).

## Clone the demo app repository

Clone the Kustomizer Git repository locally:

```bash
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

You'll be using a sample web application composed of two [podinfo](https://github.com/stefanprodan/podinfo)
instances called `frontend` and `backend`, and a redis instance called `cache`.
The web application's Kubernetes configuration is located at `./examples/demo-app`.

## Publish the app manifests 

Export the repository URL and app version:

```shell
export APP_REPO="ghcr.io/${GITHUB_USER}/kustomizer-demo-app"
export APP_VERSION="1.0.0"
```

Build and push the app manifests to GitHub container registry:

```console
$ kustomizer push artifact oci://${APP_REPO}:${APP_VERSION} \
    -k ./examples/demo-app \
	--source="$(git config --get remote.origin.url)" \
	--revision="$(git branch --show-current)/$(git rev-parse HEAD)"
building manifests...
Namespace/kustomizer-demo-app
ConfigMap/kustomizer-demo-app/redis-config-bd2fcfgt6k
Service/kustomizer-demo-app/backend
Service/kustomizer-demo-app/cache
Service/kustomizer-demo-app/frontend
Deployment/kustomizer-demo-app/backend
Deployment/kustomizer-demo-app/cache
Deployment/kustomizer-demo-app/frontend
HorizontalPodAutoscaler/kustomizer-demo-app/backend
HorizontalPodAutoscaler/kustomizer-demo-app/frontend
pushing image ghcr.io/stefanprodan/kustomizer-demo-app:1.0.0
published digest ghcr.io/stefanprodan/kustomizer-demo-app@sha256:91d2bd8e0f1620e17e9d4c308ab87903644a952969d8ff52b601be0bffdca096
```

Tag the config image as latest:

```shell
kustomizer tag artifact oci://${APP_REPO}:${APP_VERSION} latest
```

## Configure Flux to deploy the app

First install Flux on your cluster with:

```shell
flux install
```

!!! info "GitOps"
 
    For Flux to manage your cluster in a GitOps manner,
    you could use the [flux bootstrap](https://fluxcd.io/flux/installation/#bootstrap) instead of `flux install`.

Create an image pull secret for ghcr.io with:

```shell
flux create secret oci ghcr-auth \
  --url=ghcr.io \
  --username=flux \
  --password=${GITHUB_TOKEN}
```

Create a Flux `OCIRepository` for pulling the latest artifact from GitHub container registry:

```shell
flux create source oci demo-app \
  --secret-ref=ghcr-auth \
  --url=oci://${APP_REPO} \
  --tag=latest \
  --interval=1m
```

!!! info "Automated updates"

    At every minute, Flux verifies if the latest OCI artifact digest differs from the digest
    of the last downloaded artifact. When a new artifact is tagged as latest, Flux will
    detect the new version and will pull it inside the cluster.

Create a Flux `Kustomization` for applying the manifests from the artifact on the cluster:

```shell
flux create kustomization demo-app \
--source=OCIRepository/demo-app \
--prune=true \
--wait=true \
--health-check-timeout=3m \
--interval=10m
```

!!! info "Automated reconciliation"

    Every time a new version of the OCI artifact is downloaded, Flux reconciles the
    changes in the Kubernetes manifests from the artifact with the cluster state.

    During a reconciliation, Flux performs these tasks:

    * Validates the manifests against the Kubernetes API ([server-side apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/) dry run)
    * Applies the Kubernetes objects that changed in order (namespaces and other global objects first)
    * Deletes the objects that were removed from the latest artifact version
    * Waits for the changes to be successfully rollout (Helm upgrades, deployments, jobs, etc)
    * Reports the apply diff or any error as Kubernetes events

You can see the apply diff and any other Flux events with:

```sh
kubectl alpha events --for kustomization/demo-app -n flux-system
```

!!! info "Drift detection and correction"
    
    Even if nothing changed in the OCI source, Flux verifies if the cluster
    state has drifted from the desired state. If a drift is detected,
    Flux re-applies the Kubernetes objects that changed and waits for the drift 
    to be corrected. Then it emits a Kubernetes events with the list of objects 
    that were corrected.

## Promote changes to production

Assuming you're deploying the `latest` version to staging, you could introduce 
a dedicated tag for production e.g. `stable`.

On the production cluster, you'll configure Flux to reconcile the artifacts tagged as `stable`:

```shell
flux create source oci demo-app \
  --secret-ref=ghcr-auth \
  --url=oci://${APP_REPO} \
  --tag=stable \
  --interval=1m
```

To promote a version tested on staging, you would tag it as stable with:

```shell
kustomizer tag artifact oci://${APP_REPO}:${APP_VERSION} stable
```

## Automate the artifact publishing

You can automate the publishing process by running Kustomizer in CI.

Here is an example of a GitHub Actions workflow that pushes an OCI artifact
to GHCR every time there is a change to the Kubernetes configuration:

```yaml
name: publish
on:
  push:
    branches:
      - 'main'
    paths:
      - 'examples/demo-app/**'

permissions:
  contents: read
  id-token: write
  packages: write

env:
  ARTIFACT: oci://ghcr.io/${{github.repository_owner}}/${{github.event.repository.name}}

jobs:
  kustomizer:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v3
      - name: Login to GitHub Container Registry
        uses: docker/login-action@v2
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - name: Setup kustomizer
        uses: stefanprodan/kustomizer/action@main
      - name: Push
        run: |
          kustomizer push artifact ${ARTIFACT}:${GITHUB_REF_NAME} \
            -k=examples/demo-app \
          	--source=${{ github.repositoryUrl }} \
            --revision="${{ github.ref_name }}/${{ github.sha }}"
      - name: Tag latest
        run: |
          kustomizer tag artifact ${ARTIFACT}:${GITHUB_REF_NAME} latest
```

For more details on how to use Kustomizer within GitHub workflows,
please see the [GitHub Actions documentation](../github-actions.md).
