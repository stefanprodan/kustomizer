# GitHub Action

You can use Kustomizer for deploying to Kubernetes from CI. 

Here is an example with GitHub Actions:

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
      - name: Deploy
        run: |
          kustomizer apply -f ./deploy --wait --prune \
            --inventory-name=${{ github.event.repository.name }} \
            --source=${{ github.event.repository.html_url }} \
            --revision=${{ github.sha }}
```

For deploying to Kubernetes in a **GitOps** manner, take a look at [Flux](https://github.com/fluxcd/flux2).
