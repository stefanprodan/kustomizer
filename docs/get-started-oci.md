## Using container registries to distribute Kubernetes configs

This is a tutorial on how to use Kustomizer, Consign, Trivy and GitHub Container Registry
to build a secure delivery pipeline for your apps.

Publisher workflow:
- tag app release
- build and push the app image
- update the app version in the Kubernetes configuration
- build and push the app config image
- sign the app and config images

Consumer workflow:
- verify the config image provenance
- inspect the config image and extract the app container image name
- verify the app image provenance
- scan the app image for vulnerabilities
- deploy the app on Kubernetes using the manifest from the config image

## Prerequisites

Export you GitHub username:

```shell
export GITHUB_USER="YOUR-GITHUB-USERNAME"
```

Login to GitHub [Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry):

=== "docker"

    ```shell
    echo <GHCR-PUSH-TOKEN> | docker login ghcr.io -u ${GITHUB_USER} --password-stdin
    ```

=== "crane"

    ```shell
    go install github.com/google/go-containerregistry/cmd/crane@latest
    echo <GHCR-PUSH-TOKEN> | crane auth login ghcr.io -u ${GITHUB_USER} --password-stdin
    ```

Install [cosign](https://docs.sigstore.dev/cosign/installation/) and generate a key for image signing:

```shell
cosign generate-key-pair
```

## Publish and Sign the app config

Clone the Kustomizer Git repository locally:

```shell
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

Export the app config version:

```shell
export CONFIG_IMAGE="ghcr.io/${GITHUB_USER}/kustomizer-demo-app"
export CONFIG_VERSION="1.0.0"
```

Build and push the config image:

```console
$ kustomizer push artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
    -k ./testdata/oci/demo-app/
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
$ cosign sign --key cosign.key ${CONFIG_IMAGE}:${CONFIG_VERSION}
Pushing signature to: ghcr.io/stefanprodan/kustomizer-demo-app
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

List the Kubernetes manifests from the config image:

```console
$ kustomizer inspect artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION}
Artifact: oci:// ghcr.io/stefanprodan/kustomizer-demo-app@sha256:98ebc5889a1031efe84d0d27cff4a235b9fadd5378781789b8e44cbf177424cd
BuiltBy: kustomizer/v2.0.0
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
scan them for vulnerabilities with Aqua Security [Trivy](https://github.com/aquasecurity/trivy):

```console
kustomizer inspect artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
    --container-images | xargs -I {} sh -c "trivy image --severity=CRITICAL {}"
```

## Install the app

Install the application using the app config from the registry:

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

Inspect the inventory to find the artifact digest:

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
yq eval '.images[1].newTag="6.2.1"' -i ./testdata/oci/demo-app/kustomization.yaml
```

Push a new config image:

```shell
kustomizer push artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
  -k ./testdata/oci/demo-app/ 
```

Sign the new version:

```shell
cosign sign --key cosign.key ${CONFIG_IMAGE}:${CONFIG_VERSION}
```

Tag the config image as latest:

```shell
kustomizer tag artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} latest
```

## Upgrade the app

Verify the latest version:

```shell
cosign verify --key cosign.pub ${CONFIG_IMAGE}:latest
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

Patch the app config using a Kustomize strategic merge patch and apply the changes:

```console
$ kustomizer apply inventory kustomizer-demo-app --wait --prune \
  --artifact oci://${CONFIG_IMAGE}:${CONFIG_VERSION} \
  --source ${CONFIG_IMAGE} \
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

## Uninstall the app

Delete the app from your cluster:

```console
$ kustomizer delete inventory kustomizer-demo-app
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
