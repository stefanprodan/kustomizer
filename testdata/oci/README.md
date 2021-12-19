## Using container registries to distribute Kubernetes manifests

Kustomizer can package Kubernetes manifests in an OCI image and store them in a container registry,
right next to your applications' images.

Similar to Docker, Kustomizer offers commands to manage OCI artifacts:

* `kustomizer push artifact oci://<image-url>:<tag>` Push uploads Kubernetes manifests to a container registry.
* `kustomizer tag artifact oci://<image-url>:<tag> <new-tag>` Tag adds a tag for the specified OCI artifact.
* `kustomizer pull artifact oci://<image-url>:<tag>` Pull downloads Kubernetes manifests from a container registry.
* `kustomizer inspect artifact oci://<image-url>:<tag>` Inspect downloads the specified OCI artifact and prints a report of its content.

Kustomizer uses [go-containerregistry](https://github.com/google/go-containerregistry)
for interacting with container registries, and it's compatible with
Docker Hub, GHCR, ACR, ECR, GCR, Artifactory, self-hosted Docker Registry and others.
For auth, Kustomizer uses the credentials from `~/.docker/config.json`.

Assuming you've automated your application's build & push workflow using Docker,
you can extend the automation to do the same for your Kubernetes manifests that describe how your
application gets deployed.

Build a Kustomize overlay and push the multi-doc YAML to Docker Hub:

```shell
kustomizer build -k ./overlays/dist --artifact oci://docker.io/org/app-config:v1.0.0
kustomizer tag artifact oci://docker.io/org/app-config:v1.0.0 latest
```

If you're using cue, cdk8s, jsonnet, helm or any other tool that generates Kubernetes manifests,
you can pass the multi-doc YAML to Kustomizer with:

```shell
cue cmd ymldump ./deploy/app > app-config.yaml
kustomizer push artifact oci://docker.io/org/app-config:v1.0.0 -f app-config.yaml
```

Pull the app config image and diff changes with Kubernetes server-side apply dry-run:

```shell
kustomizer pull artifact oci://docker.io/org/app-config:v1.0.0 | kustomizer diff -i app -f-
```

Apply the latest app config on your cluster:

```shell
kustomizer apply -i app -a oci://docker.io/org/app-config:latest
```

## OCI Demo

This an example of how to use Kustomizer, Sigstore consign and GitHub Container Registry
to build a secure delivery pipeline for your apps.

Publisher workflow:
- tag app release
- build and push the app image
- update the app version the config manifests
- build and push the app config image
- sign both images (app and config) with cosign

Consumer workflow:
- verify the config image with cosign
- inspect the config image and extract the app container image name
- verify the app image with cosign
- deploy the app on Kubernetes using the manifest from the config image

## Prerequisites

Export you GitHub username: 

```shell
export GITHUB_USER="YOUR-GITHUB-USERNAME"
```

Login to ghcr.io ([docs](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)) with docker:

```shell
echo <GHCR-PUSH-TOKEN> | docker login ghcr.io -u ${GITHUB_USER} --password-stdin
```

If you don't have Docker installed, you can use [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane):

```shell
go install github.com/google/go-containerregistry/cmd/crane@latest
echo <GHCR-PUSH-TOKEN> | crane auth login ghcr.io -u ${GITHUB_USER} --password-stdin
```

Install [cosign](https://docs.sigstore.dev/cosign/installation/) and generate a key for image signing:

```shell
$ cosign generate-key-pair
Enter password for private key:
Enter again:
Private key written to cosign.key
Public key written to cosign.pub
```

## Publish

Clone the Kustomizer Git repository locally:

```shell
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

Export the app config version:

```shell
export CONFIG_VERSION="1.0.0"
```

Build and push the config image:

```console
$ kustomizer build -k ./testdata/oci/demo-app/ -a oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${CONFIG_VERSION} 
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

Sign the config image using your cosign private key:

```console
$ cosign sign --key cosign.key ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${CONFIG_VERSION}
Pushing signature to: ghcr.io/stefanprodan/kustomizer-demo-app
```

Tag the config image as latest:

```shell
kustomizer tag artifact oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${CONFIG_VERSION} latest
```

## Verify and Inspect

Verify the config image using your cosign public key:

```console
$ cosign verify --key cosign.pub ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${CONFIG_VERSION}
Verification for ghcr.io/stefanprodan/kustomizer-demo-app:1.0.0 --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The signatures were verified against the specified public key
  - Any certificates were verified against the Fulcio roots.

[{"critical":{"identity":{"docker-reference":"ghcr.io/stefanprodan/kustomizer-demo-app"},"image":{"docker-manifest-digest":"sha256:148c7452232a334e4843048ec41180c0c23644c30e87672bd961f31ee7ac2fca"},"type":"cosign container image signature"},"optional":null}]
```

List the Kubernetes manifests from the config image:

```console
$ kustomizer inspect artifact oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${CONFIG_VERSION}
Built by: kustomizer v1.3.0
Created at: 2021-12-15T10:05:46Z
Checksum: 5b8c45af6951e977581122b7848b490f25b43ffd44ed7a82fd574eff6aac06be
Manifests:
   Namespace/kustomizer-demo-app
   ConfigMap/kustomizer-demo-app/redis-config-bd2fcfgt6k
   Service/kustomizer-demo-app/backend
   Service/kustomizer-demo-app/cache
   Service/kustomizer-demo-app/frontend
   Deployment/kustomizer-demo-app/backend
   - ghcr.io/stefanprodan/podinfo:6.0.0
   Deployment/kustomizer-demo-app/cache
   - public.ecr.aws/docker/library/redis:6.2.0
   Deployment/kustomizer-demo-app/frontend
   - ghcr.io/stefanprodan/podinfo:6.0.0
   HorizontalPodAutoscaler/kustomizer-demo-app/backend
   HorizontalPodAutoscaler/kustomizer-demo-app/frontend
```

To list only the container images referenced in the Kubernetes manifests:

```console
$ kustomizer inspect artifact oci://ghcr.io/stefanprodan/kustomizer-demo-app:v1.0.0 --container-images
ghcr.io/stefanprodan/podinfo:6.0.0
public.ecr.aws/docker/library/redis:6.2.0
```

## Install

Install the application using the config image:

```console
$ kustomizer apply --wait --prune \
  --artifact oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${CONFIG_VERSION} \
  --inventory-name kustomizer-demo-app \
  --inventory-namespace default \
  --source ghcr.io/${GITHUB_USER}/kustomizer-demo-app \
  --revision ${CONFIG_VERSION}
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

List inventories:

```console
$ kustomizer get inventories 
NAME               	ENTRIES	SOURCE                                  	REVISION	LAST APPLIED         
kustomizer-demo-app	10     	ghcr.io/stefanprodan/kustomizer-demo-app	v1.0.0  	2021-12-16T10:33:10Z
```

## Publish updates

Bump the config version:

```shell
export CONFIG_VERSION="1.0.1"
```

Change the Redis container image tag:

```shell
yq eval '.images[1].newTag="6.2.1"' --inplace ./testdata/oci/demo-app/kustomization.yaml
```

Push a new config image:

```shell
kustomizer push artifact oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${CONFIG_VERSION}  -k ./testdata/oci/demo-app/ 
```

Sign the new version:

```shell
cosign sign --key cosign.key ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${CONFIG_VERSION}
```

Tag the config image as latest:

```shell
kustomizer tag artifact oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${CONFIG_VERSION} latest
```

## Upgrade

Verify the latest version:

```shell
cosign verify --key cosign.pub ghcr.io/${GITHUB_USER}/kustomizer-demo-app:latest
```

Pull the latest config image and diff changes:

```console
$ kustomizer pull artifact oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app | kustomizer diff -i kustomizer-demo-app --prune -f-
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

Update the app on your cluster:

```console
$ kustomizer apply --wait --prune \
  --artifact oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:latest \
  --inventory-name kustomizer-demo-app \
  --inventory-namespace default \
  --source ghcr.io/${GITHUB_USER}/kustomizer-demo-app \
  --revision ${CONFIG_VERSION}
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

## Patch upstream configs

Patch the app config using a Kustomize strategic merge patch and apply the changes:

```console
$ kustomizer apply --wait --prune \
  --artifact oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${CONFIG_VERSION} \
  --inventory-name kustomizer-demo-app \
  --inventory-namespace default \
  --source ghcr.io/${GITHUB_USER}/kustomizer-demo-app \
  --revision ${CONFIG_VERSION} \
  --patch ./testdata/patches/safe-to-evict.yaml
pulling ghcr.io/stefanprodan/kustomizer-demo-app:1.0.1
applying 10 manifest(s)...
Namespace/kustomizer-demo-app unchanged
ConfigMap/kustomizer-demo-app/redis-config-bd2fcfgt6k unchanged
Service/kustomizer-demo-app/backend unchanged
Service/kustomizer-demo-app/cache unchanged
Service/kustomizer-demo-app/frontend unchanged
Deployment/kustomizer-demo-app/backend configured
Deployment/kustomizer-demo-app/cache configured
Deployment/kustomizer-demo-app/frontend configured
HorizontalPodAutoscaler/kustomizer-demo-app/backend unchanged
HorizontalPodAutoscaler/kustomizer-demo-app/frontend unchanged
waiting for resources to become ready...
all resources are ready
```

## Uninstall

Delete the app from your cluster:

```console
$ kustomizer delete -i kustomizer-demo-app
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
