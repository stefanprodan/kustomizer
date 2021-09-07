# kustomizer

[![godev](https://img.shields.io/static/v1?label=godev&message=reference&color=00add8)](https://pkg.go.dev/github.com/stefanprodan/kustomizer)
[![e2e](https://github.com/stefanprodan/kustomizer/workflows/e2e/badge.svg)](https://github.com/stefanprodan/kustomizer/actions)
[![license](https://img.shields.io/github/license/stefanprodan/kustomizer.svg)](https://github.com/stefanprodan/kustomizer/blob/main/LICENSE)
[![release](https://img.shields.io/github/release/stefanprodan/kustomizer/all.svg)](https://github.com/stefanprodan/kustomizer/releases)

Kustomizer is a command-line utility for reconciling Kubernetes manifests and Kustomize overlays onto clusters.
Kustomizer garbage collector keeps track of the applied resources and prunes the Kubernetes
objects that were previously applied but are missing from the current revision.

Compared to `kubectl apply`, Kustomizer does things a little different:

- Validates all resources with dry-run apply, and applies only the ones with changes.
- Applies first custom resource definitions (CRDs) and namespaces, waits for them to register and only then applies the custom resources.
- Waits for the applied resources to be fully reconciled (checks the ready status of replicasets, services, ingresses, and other custom resources).
- Deletes stale objects like ConfigMaps and Secrets generated with Kustomize or other tools.

Kustomizer relies on [server-side apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/)
and requires a Kubernetes cluster **v1.18** or newer.

## Install

The Kustomizer CLI is available as a binary executable for all major platforms,
the binaries can be downloaded form GitHub [release page](https://github.com/stefanprodan/kustomizer/releases).

Install the latest release on macOS or Linux with [this script](install/README.md):

```bash
curl -s https://kustomizer.dev/install.sh | bash
```

Or from source with Go:

```sh
go install github.com/stefanprodan/kustomizer/cmd/kustomizer@latest
```

## Available Commands

The Kustomize CLI offers the following commands:

* `build`  Build scans the given path for Kubernetes manifests or Kustomize overlays and prints the YAML multi-doc to stdout.
* `apply`  Apply validates the given Kubernetes manifests or Kustomize overlays and reconciles them using server-side apply.
* `get`    Prints the content of inventories and their source revision.
* `diff`   Diff compares the local Kubernetes manifests with the in-cluster objects and prints the YAML diff to stdout.
* `delete` Delete removes the Kubernetes objects in the inventory from the cluster and waits for termination.
* `completion` Generates completion scripts for bash, fish, powershell and zsh.

## Get Started

Clone the Kustomizer Git repository locally:

```bash
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

Apply a local directory that contains Kubernetes manifests:

```console
$ kustomizer apply -f ./testdata/plain --prune --wait \
    --source="$(git ls-remote --get-url)" \
    --revision="$(git describe --always)" \
    --inventory-name=demo \
    --inventory-namespace=default

building inventory...
applying 10 manifest(s)...
Namespace/kustomizer-demo created
ServiceAccount/kustomizer-demo/demo created
ClusterRole/kustomizer-demo-read-only created
ClusterRoleBinding/kustomizer-demo-read-only created
Service/kustomizer-demo/backend created
Service/kustomizer-demo/frontend created
Deployment/kustomizer-demo/backend created
Deployment/kustomizer-demo/frontend created
HorizontalPodAutoscaler/kustomizer-demo/backend created
HorizontalPodAutoscaler/kustomizer-demo/frontend created
waiting for resources to become ready...
all resources are ready
```

Kustomizer scans the given path recursively for Kubernetes manifests in YAML format,
validates them against the cluster, applies them with server-side apply, and finally
waits for the workloads to be rolled out.

To apply Kustomize overlays, you can use `kustomizer apply -k path/to/overlay`,
for more details see `kustomizer apply --help`.

After applying the resources, Kustomizer creates an inventory.
You can list all inventories in a specific namespace with:

```console
$ kustomizer get inventories -n default

NAME	ENTRIES	SOURCE                                        	REVISION	LAST APPLIED         
demo	10     	https://github.com/stefanprodan/kustomizer.git	e44c210 	2021-09-06T16:33:08Z
```

You can list the Kubernetes objects in an inventory with:

```console
$ kustomizer get inventory demo

Inventory: default/demo
LastApplied: 2021-09-06T16:33:08Z
Source: https://github.com/stefanprodan/kustomizer.git
Revision: e44c210
Entries:
- Namespace/kustomizer-demo
- ServiceAccount/kustomizer-demo/demo
- ClusterRole/kustomizer-demo-read-only
- ClusterRoleBinding/kustomizer-demo-read-only
- Service/kustomizer-demo/backend
- Service/kustomizer-demo/frontend
- Deployment/kustomizer-demo/backend
- Deployment/kustomizer-demo/frontend
- HorizontalPodAutoscaler/kustomizer-demo/backend
- HorizontalPodAutoscaler/kustomizer-demo/frontend
```

The inventory records are used to track which objects are subject to garbage collection.
The inventory is persistent on the cluster as a ConfigMap.

Change the min replicas of the `backend` HPA and remove the `frontend` and the `rbac` manifests from the local dir:

```bash
rm -rf testdata/plain/frontend
rm -rf testdata/plain/common/rbac.yaml
```

Preview the changes using diff:

```console
$ kustomizer diff -i demo -f ./testdata/plain/ --prune

► HorizontalPodAutoscaler/kustomizer-demo/backend drifted
  (
  	"""
  	... // 18 identical lines
  	        type: Utilization
  	    type: Resource
- 	  minReplicas: 2
+ 	  minReplicas: 1
  	  scaleTargetRef:
  	    apiVersion: apps/v1
  	... // 32 identical lines
  	"""
  )

► ClusterRole/kustomizer-demo-read-only deleted
► ClusterRoleBinding/kustomizer-demo-read-only deleted
► Service/kustomizer-demo/frontend deleted
► Deployment/kustomizer-demo/frontend deleted
► HorizontalPodAutoscaler/kustomizer-demo/frontend deleted
```

Rerun the apply command:

```console
$ kustomizer apply -i demo -f testdata/plain/ --prune --wait

building inventory...
applying 5 manifest(s)...
Namespace/kustomizer-demo unchanged
ServiceAccount/kustomizer-demo/demo unchanged
Service/kustomizer-demo/backend unchanged
Deployment/kustomizer-demo/backend unchanged
HorizontalPodAutoscaler/kustomizer-demo/backend configured
HorizontalPodAutoscaler/kustomizer-demo/frontend deleted
Deployment/kustomizer-demo/frontend deleted
Service/kustomizer-demo/frontend deleted
ClusterRoleBinding/kustomizer-demo-read-only deleted
ClusterRole/kustomizer-demo-read-only deleted
waiting for resources to become ready...
all resources are ready
```

After applying the resources, Kustomizer removes the Kubernetes objects that are not present in the current inventory.
Kustomizer garbage collector deletes the namespaced objects first then it removes the non-namspaced ones.

Delete all the Kubernetes objects belonging to an inventory including the inventory ConfigMap:

```console
$ kustomizer delete -i demo --wait

retrieving inventory...
deleting 5 manifest(s)...
HorizontalPodAutoscaler/kustomizer-demo/backend deleted
Deployment/kustomizer-demo/backend deleted
Service/kustomizer-demo/backend deleted
ServiceAccount/kustomizer-demo/demo deleted
Namespace/kustomizer-demo deleted
ConfigMap/default/demo deleted
waiting for resources to be terminated...
all resources have been deleted
```

## CIOps

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

For deploying to Kubernetes in a **GitOps** manner,
take a look at [Flux](https://github.com/fluxcd/flux2).

## Motivation

If you got so far you may wander how is Kustomizer different to running:

```bash
kubectl apply -k ./my-app --prune -l app=my-app
```

The pruning feature in kubectl while still experimental has many downsides, most notable is that pruning
requires an account that can query Kubernetes API for non-namespaced objects,
this means you can't run prune under a user with restricted access to cluster wide objects.
Another downside is the fact that pruning can delete non-namespaced objects outside of the apply scope.
If you want to prune custom resources, then you need to pass the group/version/kind to prune-whitelist
and maintain a list per kustomization. 

Kustomizer can reliably detect the objects that were previously applied but 
are missing from the current inventory. For namespaced objects, Kustomizer runs the delete commands
scoped to a namespace, this way an account that doesn't have a cluster role binding can prune
objects in the namespaces it owns.

## Contributing

Kustomizer is Apache 2.0 licensed and accepts contributions via GitHub pull requests.
