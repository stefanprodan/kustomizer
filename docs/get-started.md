# Get Started

This guide shows you how to install Kustomizer and how to deploy a sample application to a Kubernetes cluster.
To follow the guide you'll need a Kubernetes cluster version 1.20 or newer.

## Install the Kustomizer CLI

Install the latest release on macOS or Linux with:

```bash
curl -s https://kustomizer.dev/install.sh | sudo bash
```

For other installation methods, see the CLI [install documentation](install.md).

To connect to Kubernetes API, Kustomizer uses the current context from `~/.kube/config`.
You can set a different context with the `--context` flag.
You can also specify a different kubeconfig with `--kubeconfig` or with the `KUBECONFIG` env var.

## Clone the git repository

Clone the Kustomizer Git repository locally:

```bash
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

## Create resources

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

## List inventories

After applying the resources, Kustomizer creates an inventory.
You can list all inventories in a specific namespace with:

```console
$ kustomizer get inventories -n default

NAME	ENTRIES	SOURCE                                        	REVISION	LAST APPLIED         
demo	10     	https://github.com/stefanprodan/kustomizer.git	e44c210 	2021-12-06T16:33:08Z
```

## List resources

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

## Diff changes

Change the max replicas of the `backend` HPA and remove the `frontend` and the `rbac` manifests from the local dir:

```bash
rm -rf testdata/plain/frontend
rm -rf testdata/plain/common/rbac.yaml
```

Preview the changes using diff:

```console
$ kustomizer diff -i demo -f ./testdata/plain/ --prune

► HorizontalPodAutoscaler/kustomizer-demo/backend drifted
@@ -11,7 +11,7 @@
         resourceVersion: "572967"
         uid: ca841aab-46b9-4a51-8e44-3cf5f615791d
     spec:
-        maxReplicas: 4
+        maxReplicas: 5
         metrics:
             - resource:
                 name: cpu
► ClusterRole/kustomizer-demo-read-only deleted
► ClusterRoleBinding/kustomizer-demo-read-only deleted
► Service/kustomizer-demo/frontend deleted
► Deployment/kustomizer-demo/frontend deleted
► HorizontalPodAutoscaler/kustomizer-demo/frontend deleted
```

Note that when diffing Kubernetes secrets, kustomizer masks the secret values in the output.

## Update resources

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

## Delete resources

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
