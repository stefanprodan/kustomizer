# Kustomizer GitHub Action

Usage:

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
          version: 1.0.0
```

Deploy to Kubernetes with:

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
      - name: Setup kubeconfig
        uses: azure/k8s-set-context@v1
        with:
          kubeconfig: ${{ secrets.KUBE_CONFIG }}
      - name: Install Kustomizer
        uses: stefanprodan/kustomizer/action@main
      - name: Validate
        run: |
          kustomizer build -f ./deploy | kubeval -
      - name: Deploy
        run: |
          kustomizer apply -f ./deploy --wait --prune \
            --inventory-name=${{ github.event.repository.name }} \
            --source=${{ github.event.repository.html_url }} \
            --revision=${{ github.sha }}
```
