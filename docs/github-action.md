# Kustomizer GitHub Action

You can use Kustomizer to push OCI artifacts to container registries and deploy to Kubernetes from CI.

## Usage

```yaml
    steps:
      - name: Setup Kustomizer CLI
        uses: stefanprodan/kustomizer/action@main
      - name: Run Kustomizer commands
        run: kustomizer -v
```

The latest stable version of the `kustomizer` binary is downloaded from
GitHub [releases](https://github.com/stefanprodan/kustomizer/releases)
and placed at `/usr/local/bin/kustomizer`.

Note that this action can only be used on GitHub **Linux** runners.
You can change the arch (defaults to `amd64`) with:

```yaml
    steps:
      - name: Setup Kustomizer CLI
        uses: stefanprodan/kustomizer/action@main
        with:
          arch: arm64 # can be amd64 or arm64
```

You can download a specific version with:

```yaml
    steps:
      - name: Setup Kustomizer CLI
        uses: stefanprodan/kustomizer/action@main
        with:
          version: 2.0.0
```

## Publish artifacts to GHCR

Example of publishing OCI artifacts to GitHub Container Registry:

```yaml
name: publish
on:
  push:
    tag:
      - 'v*'

env:
  ARTIFACT: oci://ghcr.io//${{github.repository_owner}}/${{github.event.repository.name}}

jobs:
  kustomizer:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v2
      - name: Login to GitHub Container Registry
        uses: docker/login-action@v1
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GHCR_TOKEN }}
      - name: Setup kustomizer
        uses: stefanprodan/kustomizer/action@main
      - name: Push
        run: |
          kustomizer push artifact ${ARTIFACT}:${GITHUB_REF_NAME} -f ./deploy
      - name: Tag latest
        run: |
          kustomizer tag artifact ${ARTIFACT}:${GITHUB_REF_NAME} latest
```

## Deploy to Kubernetes from GHCR

Example of applying Kubernetes manifests from an OCI artifact:

```yaml
name: deploy
on:
  workflow_dispatch:
    inputs:
      name:
        description: 'Tag to deploy'
        required: true
        default: 'latest'

env:
  ARTIFACT: oci://ghcr.io//${{github.repository_owner}}/${{github.event.repository.name}}

jobs:
  kustomizer:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v2
      - name: Setup kubeconfig
        uses: azure/k8s-set-context@v1
        with:
          kubeconfig: ${{ secrets.KUBE_CONFIG }}
      - name: Login to GitHub Container Registry
        uses: docker/login-action@v1
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GHCR_TOKEN }}
      - name: Setup kustomizer
        uses: stefanprodan/kustomizer/action@main
      - name: Deploy
        run: |
          kustomizer apply inventory ${{ github.event.repository.name }} \
            --artifact ${ARTIFACT}:${{ github.event.inputs.name }} \
            --prune --wait
```

## Deploy to Kubernetes from Git

Example of applying Kubernetes manifests from a Git repository:

```yaml
name: deploy
on:
  push:
    branches:
      - 'main'

jobs:
  kustomizer:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v2
      - name: Setup kubeconfig
        uses: azure/k8s-set-context@v1
        with:
          kubeconfig: ${{ secrets.KUBE_CONFIG }}
      - name: Setup kustomizer
        uses: stefanprodan/kustomizer/action@main
      - name: Diff
        run: |
          kustomizer diff inventory ${{ github.event.repository.name }} \
            -f ./deploy --prune
      - name: Deploy
        run: |
          kustomizer apply inventory ${{ github.event.repository.name }} \
            --source=${{ github.event.repository.html_url }} \
            --revision=${{ github.sha }} \
            -f ./deploy --prune --wait
```
