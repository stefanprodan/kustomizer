# Secure your Kubernetes supply chain with Kustomizer and Cosign

Kustomizer offers a way to distribute Kubernetes configuration as OCI artifacts.
This means you can store your application configuration in the same 
registry where your application container images are.

[Cosign](https://github.com/sigstore/cosign) is tool for signing and verifying OCI artifacts.
You can use Cosign to sign both your application container images (created with Docker)
and the config images (created with Kustomizer).

!!! important "Delivery workflow"

    As an application publisher you:

    - release a new app version
    - build and push the app container image
    - update the app version in the Kubernetes configuration
    - build and push the app config image
    - sign the app and config images

    As a Kubernetes operator you:

    - verify the config image signature
    - inspect the config image and extract the app container image name
    - verify the container image signature
    - scan the container image for vulnerabilities
    - deploy the app onto clusters using the Kubernetes manifests from the config image    

What follows is a guide on how to use Kustomizer, Cosign, Trivy and
GitHub Container Registry to build a secure delivery pipeline for a sample application.

## Prerequisites

To follow this guide you'll need a GitHub account and a Kubernetes cluster version 1.20 or newer.

Install [cosign](https://docs.sigstore.dev/cosign/installation/),
[trivy](https://github.com/aquasecurity/trivy), [yq](https://github.com/mikefarah/yq)
and Kustomizer with Homebrew:

```shell
brew install cosign yq aquasecurity/trivy/trivy stefanprodan/tap/kustomizer
```

Generate a cosign key pair for image signing with:

```shell
cosign generate-key-pair
```

## Login to GitHub Container Registry

Export you GitHub username:

```shell
export GITHUB_USER="YOUR-GITHUB-USERNAME"
```

Generate a [personal access token (PAT)](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
with read and write access to GitHub Container Registry. 

Use the PAT to sign in to the container registry service at ghcr.io:

```console
$ echo $CR_PAT | docker login ghcr.io -u ${GITHUB_USER} --password-stdin
> Login Succeeded
```

## Clone the demo app repository

Clone the Kustomizer Git repository locally:

```bash
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

You'll be using a sample web application composed of two [podinfo](https://github.com/stefanprodan/podinfo)
instances called `frontend` and `backend`, and a redis instance called `cache`.
The web application's Kubernetes configuration is located at `./examples/demo-app`.

## Publish and Sign the config image

Export the config image URL and version:

```shell
export CONFIG_IMAGE="ghcr.io/${GITHUB_USER}/kustomizer-demo-app"
export CONFIG_VERSION="1.0.0"
```

Export your cosign private key password:

```shell
COSIGN_PASSWORD=<YOUR-PASS>
```

Push and sign the config image:

```console
$ kustomizer push artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
    -k ./examples/demo-app \
    --sign --cosign-key cosign.key
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
cosign pushing signature to: ghcr.io/stefanprodan/kustomizer-demo-app
```

Tag the config image as latest:

```shell
kustomizer tag artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} latest
```

## Verify and Scan the app

Verify the config image using your cosign public key:

```console
$ cosign verify --key cosign.pub ${CONFIG_IMAGE}:${CONFIG_VERSION}
Verification for ghcr.io/stefanprodan/kustomizer-demo-app:1.0.0 --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The signatures were verified against the specified public key
  - Any certificates were verified against the Fulcio roots.

[{"critical":{"identity":{"docker-reference":"ghcr.io/stefanprodan/kustomizer-demo-app"},"image":{"docker-manifest-digest":"sha256:148c7452232a334e4843048ec41180c0c23644c30e87672bd961f31ee7ac2fca"},"type":"cosign container image signature"},"optional":null}]
```

Verify the image using your cosign public key and list the Kubernetes manifests from the config image:

```console
$ kustomizer inspect artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
    --verify --cosign-key cosign.pub
Artifact: oci:// ghcr.io/stefanprodan/kustomizer-demo-app@sha256:98ebc5889a1031efe84d0d27cff4a235b9fadd5378781789b8e44cbf177424cd
BuiltBy: kustomizer/v2.0.0
VerifiedBy: cosign
CreatedAt: 2021-12-15T10:05:46Z
Resources:
- Namespace/kustomizer-demo-app
- ConfigMap/kustomizer-demo-app/redis-config-bd2fcfgt6k
- Service/kustomizer-demo-app/backend
- Service/kustomizer-demo-app/cache
- Service/kustomizer-demo-app/frontend
- Deployment/kustomizer-demo-app/backend
  - ghcr.io/stefanprodan/podinfo:6.0.0
- Deployment/kustomizer-demo-app/cache
  - public.ecr.aws/docker/library/redis:6.2.0
- Deployment/kustomizer-demo-app/frontend
  - ghcr.io/stefanprodan/podinfo:6.0.0
- HorizontalPodAutoscaler/kustomizer-demo-app/backend
- HorizontalPodAutoscaler/kustomizer-demo-app/frontend
```

You can list the container images referenced in the Kubernetes manifests and
scan them for vulnerabilities with [trivy](https://github.com/aquasecurity/trivy):

```console
kustomizer inspect artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
    --container-images | xargs -I {} sh -c "trivy image --severity=CRITICAL {}"
```

## Install the app

Install the demo application using the manifests from the config image:

```console
$ kustomizer apply inventory kustomizer-demo-app --wait --prune \
  --artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
  --source ${CONFIG_IMAGE} \
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
$ kustomizer get inventories -n default
NAME               	ENTRIES	SOURCE                                  	REVISION	LAST APPLIED         
kustomizer-demo-app	10     	ghcr.io/stefanprodan/kustomizer-demo-app	v1.0.0  	2021-12-16T10:33:10Z
```

Inspect the inventory to find the config image digest:

```console
$ kustomizer inspect inv kustomizer-demo-app -n default
Inventory: default/kustomizer-demo-app
LastAppliedAt: 2021-12-20T23:05:45Z
Source: oci://ghcr.io/stefanprodan/kustomizer-demo-app
Revision: v1.0.0
Artifacts:
- oci://ghcr.io/stefanprodan/kustomizer-demo-app@sha256:d47a1734843b7144b6fb2f74d525abaaa63ca3ab8c0c82dc748acd541332df9f
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

## Publish app updates

Bump the config version:

```shell
export CONFIG_VERSION="1.0.1"
```

Change the Redis container image tag with [yq](https://github.com/mikefarah/yq):

```shell
yq eval '.images[1].newTag="6.2.1"' -i ./examples/demo-app/kustomization.yaml
```

Push a new config image:

```shell
kustomizer push artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
  -k ./examples/demo-app/ --sign --cosign-key cosign.key
```

Tag the config image as latest:

```shell
kustomizer tag artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} latest
```

## Upgrade the app

Verify the latest version:

```shell
kustomizer inspect artifact oci://${CONFIG_IMAGE}:latest \
    --verify --cosign-key cosign.pub
```

Pull the latest config image and diff changes:

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

Update the app on your cluster:

```console
$ kustomizer apply inventory kustomizer-demo-app --wait --prune \
  --artifact oci://${CONFIG_IMAGE}:latest \
  --source ${CONFIG_IMAGE} \
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

At apply time, you can modify the manifests using
[kustomize patches](https://github.com/kubernetes-sigs/kustomize/blob/master/examples/patchMultipleObjects.md).

Mark the application pods as safe to evict by the cluster autoscaler with:

```console
$ kustomizer apply inventory kustomizer-demo-app --wait --prune \
  --artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
  --source ${CONFIG_IMAGE} \
  --revision ${CONFIG_VERSION} \
  --patch ./examples/patches/safe-to-evict.yaml
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

## Uninstall the app

Delete the app and its inventory from your cluster:

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
