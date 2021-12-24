# Get Started

This guide shows you how to use Kustomizer to publish and deploy a sample application.

To follow this guide you'll need a GitHub account and a Kubernetes cluster version 1.20 or newer.

## Prerequisites

### Install the Kustomizer CLI

Install the latest release on macOS or Linux with:

```shell
brew install stefanprodan/tap/kustomizer
```

For other installation methods, see the CLI [install documentation](install.md).

### Login to GitHub Container Registry

Generate a [personal access token (PAT)](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
with read and write access to GitHub Container Registry (GHCR).

Export you GitHub username:

```shell
export GITHUB_USER="YOUR-GITHUB-USERNAME"
```

Use the PAT to sign in to the container registry service at ghcr.io:

```shell
echo <PAT> | docker login ghcr.io -u ${GITHUB_USER} --password-stdin
```

!!! info "Other container registries"

    Besides GHCR, Kustomizer is compatible with Docker Hub, ACR, ECR, GCR, Artifactory,
    self-hosted Docker Registry v2 and any other registry that conforms
    to the [Open Container Initiative](https://opencontainers.org/).

## Publish the app config

You'll be using a sample web application composed of two [podinfo](https://github.com/stefanprodan/podinfo)
instances called `frontend` and `backend`, and a redis instance called `cache`.

### Clone the demo app repository

Clone the Kustomizer Git repository locally:

```shell
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

The sample application configuration is a Kustomize overlay located at `./examples/demo-app`.

### Build and push the app config to GHCR

Export the config image URL and version:

```shell
export CONFIG_IMAGE="ghcr.io/${GITHUB_USER}/kustomizer-demo-app"
export CONFIG_VERSION="1.0.0"
```

Push the config image to your GHCR repository:

```console
$ kustomizer push artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
    -k ./examples/demo-app/
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

With the above command, Kustomizer builds the Kustomize overlay at `./examples/demo-app/`,
packages the resulting multi-doc YAML as an OCI artifact and pushes the image to GHCR.

After you run the command, the image repository can be accessed at
`https://github.com/users/<YOUR USERNAME>/packages`.

!!! info "Generating Kubernetes manifests"

    Besides Kustomize overlays, Kustomizer can package plain Kuberentes manifests.
    
    If you're using [Cue](https://cuelang.org/) to define your app config, export the 
    manifests to a multi-doc YAML file and pass the path to Kustomizer with `-f <path to .yaml>` e.g.:
    
    ```shell
    cue export -p my-app -o yaml > my-app.yaml
    kustomizer push artifact oci://docker.io/my-org/my-app:1.0.0 -f my-app.yaml
    ```

### Publish app updates

Change the Redis container image tag with [yq](https://github.com/mikefarah/yq):

```shell
yq eval '.images[1].newTag="6.2.1"' -i ./examples/demo-app/kustomization.yaml
```

Bump the config version:

```shell
export CONFIG_VERSION="1.0.1"
```

Push the new version to GHCR:

```shell
kustomizer push artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
  -k ./examples/demo-app/ 
```

Tag the config image as latest:

```shell
kustomizer tag artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} latest
```

## Deploy the app on Kubernetes

### Install the app

Apply the Kubernetes configuration from GHCR:

```console
$ kustomizer apply inventory kustomizer-demo-app --wait --prune \
  --artifact oci://${CONFIG_IMAGE}:1.0.0 \
  --source ${CONFIG_IMAGE} \
  --revision 1.0.0
pulling ghcr.io/stefanprodan/kustomizer-demo-app:1.0.0
applying 10 manifest(s)...
Namespace/kustomizer-demo-app created
ConfigMap/kustomizer-demo-app/redis-config-bd2fcfgt6k created
Service/kustomizer-demo-app/backend created
Service/kustomizer-demo-app/cache created
Service/kustomizer-demo-app/frontend created
Deployment/kustomizer-demo-app/backend created
Deployment/kustomizer-demo-app/cache created
Deployment/kustomizer-demo-app/frontend created
HorizontalPodAutoscaler/kustomizer-demo-app/backend created
HorizontalPodAutoscaler/kustomizer-demo-app/frontend created
waiting for resources to become ready...
all resources are ready
```

With the above command, Kustomizer pulls the artifact from GHCR,
extracts the Kubernetes resources and validates them against the Kubernetes API,
applies the resources with server-side apply,
then waits for the workloads to become ready.

List inventories with:

```console
$ kustomizer get inventories -n default
NAME               	ENTRIES	SOURCE                                  	REVISION	LAST APPLIED         
kustomizer-demo-app	10     	ghcr.io/stefanprodan/kustomizer-demo-app	1.0.0  	  	2021-12-16T10:33:10Z
```

At apply time, Kustomizer creates an inventory to keep track of the set of resources applied together.
The inventory is stored inside the cluster in a `ConfigMap` object and contains metadata
such as the resources IDs, provenance and revision.

Inspect the inventory with:

```console
$ kustomizer inspect inv kustomizer-demo-app -n default
Inventory: default/kustomizer-demo-app
LastAppliedAt: 2021-12-20T23:05:45Z
Source: oci://ghcr.io/stefanprodan/kustomizer-demo-app
Revision: 1.0.0
Artifacts:
- oci://ghcr.io/stefanprodan/kustomizer-demo-app@sha256:19ded6e1dbe3eb859bd0f0fa6aa1960f6975097af8f19e252b951cf3e9e9e6e2
Resources:
- Namespace/kustomizer-demo-app
- ConfigMap/kustomizer-demo-app/redis-config-bd2fcfgt6k
- Service/kustomizer-demo-app/backend
- Service/kustomizer-demo-app/cache
- Service/kustomizer-demo-app/frontend
- Deployment/kustomizer-demo-app/backend
- Deployment/kustomizer-demo-app/cache
- Deployment/kustomizer-demo-app/frontend
- HorizontalPodAutoscaler/kustomizer-demo-app/backend
- HorizontalPodAutoscaler/kustomizer-demo-app/frontend
```

At apply time, Kustomizer saves the artifact URL and the image SHA-2 digest in the inventory.

!!! info "Using digests"

    For deterministic and repeatable apply operations, you can specify the digest instead of the image tag e.g.:
    
    ```shell
    kustomizer apply inventory kustomizer-demo-app --wait --prune \
      --artifact oci://ghcr.io/stefanprodan/kustomizer-demo-app@sha256:19ded6e1dbe3eb859bd0f0fa6aa1960f6975097af8f19e252b951cf3e9e9e6e2
    ```

### Update the app

Pull the latest version of the config image and diff changes:

```console
$ kustomizer diff inventory kustomizer-demo-app --prune \
    --artifact oci://${CONFIG_IMAGE}:latest 
► Deployment/kustomizer-demo-app/cache drifted
@@ -5,7 +5,7 @@
     deployment.kubernetes.io/revision: "1"
     env: demo
   creationTimestamp: "2021-12-13T19:50:26Z"
-  generation: 1
+  generation: 2
   labels:
     app.kubernetes.io/instance: webapp
     inventory.kustomizer.dev/name: kustomizer-demo-app
@@ -36,7 +36,7 @@
       - command:
         - redis-server
         - /redis-master/redis.conf
-        image: public.ecr.aws/docker/library/redis:6.2.0
+        image: public.ecr.aws/docker/library/redis:6.2.1
         imagePullPolicy: IfNotPresent
         livenessProbe:
           failureThreshold: 3
```

Apply the latest configuration on your cluster:

```console
$ kustomizer apply inventory kustomizer-demo-app --wait --prune \
  --artifact oci://${CONFIG_IMAGE}:latest \
  --source ${CONFIG_IMAGE} \
  --revision 1.0.1
pulling ghcr.io/stefanprodan/kustomizer-demo-app:latest
applying 10 manifest(s)...
Namespace/kustomizer-demo-app unchanged
ConfigMap/kustomizer-demo-app/redis-config-bd2fcfgt6k unchanged
Service/kustomizer-demo-app/backend unchanged
Service/kustomizer-demo-app/cache unchanged
Service/kustomizer-demo-app/frontend unchanged
Deployment/kustomizer-demo-app/backend unchanged
Deployment/kustomizer-demo-app/cache configured
Deployment/kustomizer-demo-app/frontend unchanged
HorizontalPodAutoscaler/kustomizer-demo-app/backend unchanged
HorizontalPodAutoscaler/kustomizer-demo-app/frontend unchanged
waiting for resources to become ready...
all resources are ready
```

### Uninstall the app

Delete all the Kubernetes resources belonging to an inventory including the inventory storage:

```console
$ kustomizer delete inventory kustomizer-demo-app --wait
retrieving inventory...
deleting 10 manifest(s)...
HorizontalPodAutoscaler/kustomizer-demo-app/frontend deleted
HorizontalPodAutoscaler/kustomizer-demo-app/backend deleted
Deployment/kustomizer-demo-app/frontend deleted
Deployment/kustomizer-demo-app/cache deleted
Deployment/kustomizer-demo-app/backend deleted
Service/kustomizer-demo-app/frontend deleted
Service/kustomizer-demo-app/cache deleted
Service/kustomizer-demo-app/backend deleted
ConfigMap/kustomizer-demo-app/redis-config-bd2fcfgt6k deleted
Namespace/kustomizer-demo-app deleted
ConfigMap/default/kustomizer-demo-app deleted
waiting for resources to be terminated...
all resources have been deleted
```
