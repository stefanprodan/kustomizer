# Kustomizer GitHub Actions

You can use Kustomizer to push artifacts to container registries and
deploy to Kubernetes from your GitHub workflows.

## Usage

To run Kustomizer commands on GitHub **Linux** runners,
add the following steps to your GitHub workflow:

```yaml
    steps:
      - name: Setup Kustomizer CLI
        uses: stefanprodan/kustomizer/action@main
        with:
          version: 2.0.0 # defaults to latest
          arch: amd64 # can be amd64 or arm64
      - name: Run Kustomizer commands
        run: kustomizer -v
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
          password: ${{ secrets.GHCR_TOKEN }}
      - name: Setup kustomizer
        uses: stefanprodan/kustomizer/action@main
      - name: Push
        run: |
          kustomizer push artifact ${ARTIFACT}:${{ github.ref_name }} -f ./deploy \
          	--source=${{ github.repositoryUrl }} \
            --revision="${{ github.ref_name }}/${{ github.sha }}"
      - name: Tag latest
        run: |
          kustomizer tag artifact ${ARTIFACT}:${GITHUB_REF_NAME} latest
```

## Publish signed artifacts

Example of publishing signed artifacts using cosgin [keyless signatures](https://github.com/sigstore/cosign/blob/main/KEYLESS.md)
and GitHub OIDC:

```yaml
name: publish
on:
  push:
    tag:
      - 'v*'

permissions:
  contents: read # needed for checkout
  id-token: write # needed for keyless signing
  packages: write # needed for GHCR access

env:
  ARTIFACT: oci://ghcr.io/${{github.repository_owner}}/${{github.event.repository.name}}

jobs:
  kustomizer:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v3
      - name: Setup cosign
        uses: sigstore/cosign-installer@main
      - name: Setup kustomizer
        uses: stefanprodan/kustomizer/action@main
      - name: Login to GitHub Container Registry
        uses: docker/login-action@v2
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - name: Push and sign
        run: |
          kustomizer push artifact ${ARTIFACT}:${GITHUB_REF_NAME} -f ./deploy --sign \
          	--source=${{ github.repositoryUrl }} \
            --revision="${{ github.ref_name }}/${{ github.sha }}"
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
  ARTIFACT: oci://ghcr.io/${{github.repository_owner}}/${{github.event.repository.name}}

jobs:
  kustomizer:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v3
      - name: Setup kubeconfig
        uses: azure/k8s-set-context@v1
        with:
          kubeconfig: ${{ secrets.KUBE_CONFIG }}
      - name: Login to GitHub Container Registry
        uses: docker/login-action@v2
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GHCR_TOKEN }}
      - name: Setup cosign
        uses: sigstore/cosign-installer@main
      - name: Setup kustomizer
        uses: stefanprodan/kustomizer/action@main
      - name: Verify signature
        run: |
          kustomizer inspect artifact ${ARTIFACT}:${{ github.event.inputs.name }} --verify
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
        uses: actions/checkout@v3
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

