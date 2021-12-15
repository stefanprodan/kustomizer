# Kustomizer OCI Demo

Export you GitHub username: 

```shell
export GITHUB_USER="GITHUB-USER"
```

Login to ghcr.io ([docs](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)) with docker:

```shell
echo <GHCR-PUSH-TOKEN> | docker login ghcr.io -u ${GITHUB_USER} --password-stdin
```

Generate a key for image signing:

```shell
$ cosign generate-key-pair
Enter password for private key:
Enter again:
Private key written to cosign.key
Public key written to cosign.pub
```

Clone the Kustomizer Git repository locally:

```shell
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

Export app version:

```shell
export DEPLOY_VERSION="1.0.0"
```

Build and push image:

```console
$ kustomizer build -k ./testdata/oci/demo-app/ -a oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${DEPLOY_VERSION} 
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

Sign image using the private key:

```console
$ cosign sign --key cosign.key ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${DEPLOY_VERSION}
Pushing signature to: ghcr.io/stefanprodan/kustomizer-demo-app
```

Verify image using the public key:

```console
$ cosign verify --key cosign.pub ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${DEPLOY_VERSION}
Verification for ghcr.io/stefanprodan/kustomizer-demo-app:1.0.0 --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The signatures were verified against the specified public key
  - Any certificates were verified against the Fulcio roots.

[{"critical":{"identity":{"docker-reference":"ghcr.io/stefanprodan/kustomizer-demo-app"},"image":{"docker-manifest-digest":"sha256:148c7452232a334e4843048ec41180c0c23644c30e87672bd961f31ee7ac2fca"},"type":"cosign container image signature"},"optional":null}]
```

List the app manifests:

```console
$ kustomizer inspect oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${DEPLOY_VERSION}
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

Install app:

```console
$ kustomizer apply --wait --prune \
  --artifact oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${DEPLOY_VERSION} \
  --inventory-name kustomizer-demo-app \
  --inventory-namespace default \
  --source ghcr.io/${GITHUB_USER}/kustomizer-demo-app \
  --revision ${DEPLOY_VERSION}
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

Bump deploy version:

```shell
export DEPLOY_VERSION="1.0.1"
```

Change the Redis image tag:

```shell
yq eval '.images[1].newTag="6.2.1"' --inplace ./testdata/oci/demo-app/kustomization.yaml
```

Push a new image:

```shell
kustomizer push -a oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${DEPLOY_VERSION}  -k ./testdata/oci/demo-app/ 
```

Diff changes:

```console
$ kustomizer pull oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${DEPLOY_VERSION} | kustomizer diff -i kustomizer-demo-app --prune -f-
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

Update app:

```console
$ kustomizer apply --wait --prune \
  --artifact oci://ghcr.io/${GITHUB_USER}/kustomizer-demo-app:${DEPLOY_VERSION} \
  --inventory-name kustomizer-demo-app \
  --inventory-namespace default \
  --source ghcr.io/${GITHUB_USER}/kustomizer-demo-app \
  --revision ${DEPLOY_VERSION}
pulling ghcr.io/stefanprodan/kustomizer-demo-app:1.0.1
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
