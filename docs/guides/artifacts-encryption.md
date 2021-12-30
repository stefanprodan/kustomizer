# Encryption at rest with Kustomizer and Age

Kustomize has builtin support for encrypting and decrypting Kubernetes configuration (packaged as OCI artifacts)
using Actually Good Encryption (age) asymmetric keys.
[Age](https://github.com/FiloSottile/age) is a modern and secure encryption tool 
with small explicit keys that is a viable alternative to [PGP](https://en.wikipedia.org/wiki/Pretty_Good_Privacy).

This guide shows you how to securely distribute sensitive Kubernetes configuration to trusted consumers.
When publishing OCI artifacts to a container registry,
you can opt to encrypt the artifacts content using your consumers public keys.

## Before you begin

- Install the Kustomizer CLI by the following instructions in the [Installation guide](../install.md).
- Install the age key generator CLI e.g. `brew install age`.

## Key management

Assuming you want to publish sensitive Kubernetes configuration that can be accessed only by a
set of trusted users, first you'll need to acquire their public keys.

### Generate key pairs

Generate a X25519 key pair using age keygen CLI:

```shell
age-keygen -o id_age
```

Extract the public key to a separate file:

```shell
age-keygen -y id_age > id_age.pub
```

### Create the recipients file

Collect the public keys from your users and save them in a file (one key per line):

```shell
touch recipients.txt
cat user1/id_age.pub >> recipients.txt
cat user2/id_age.pub >> recipients.txt
```

For testing purposes, add your own public key to the recipients file:

```shell
cat id_age.pub >> recipients.txt
```

## Publish encrypted artifacts

To encrypt Kubernetes configuration with your users' public keys, you can point Kustomizer 
at the recipients file when pushing artifacts to a container registry:

```shell
kustomizer push artifact oci://ghcr.io/my-org/my-app:1.0.0 \
-k ./examples/demo-app \
--age-recipients ./recipients.txt
```

Now if you try to inspect the artifact, Kustomizer will fail to access the artifact content:

```console
$ kustomizer inspect artifact oci://ghcr.io/my-org/my-app:1.0.0
✗ pulling ghcr.io/my-org/my-app:1.0.0 failed: encrypted artifact, you need to supply a private key for decryption
```

## Consume encrypted artifacts

To make use of encrypted artifacts, you'll need to point Kustomizer to a local file with
one or more private keys (age identities).

To inspect an artifact encrypted with your public key:

```console
$ kustomizer inspect artifact oci://ghcr.io/my-org/my-app:1.0.0 \
    --age-identities ./id_age 
Artifact: oci://ghcr.io/my-org/my-app@sha256:1801d42d5459e81119dad543a7f1080ed2aadc92dcbb7c9dabf282692d6bf29d
BuiltBy: kustomizer/v2.0.0
CreatedAt: 2021-12-29T08:35:40Z
EncryptedWith: age-encryption.org/v1
Checksum: 5b8c45af6951e977581122b7848b490f25b43ffd44ed7a82fd574eff6aac06be
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

All Kustomizer commands that pull artifacts from container registries expose the 
`--age-identities` flag, e.g.:

```shell
kustomizer apply inventory sensitive-app --wait --prune \
  --artifact oci://ghcr.io/my-org/app-frontend:1.0.0 \
  --artifact oci://ghcr.io/my-org/app-backend:1.0.0 \
  --age-identities ./id_age
```
