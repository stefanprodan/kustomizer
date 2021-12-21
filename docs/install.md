# Install

The Kustomizer CLI is available as a binary executable for all major platforms,
the binaries can be downloaded form GitHub [release page](https://github.com/stefanprodan/kustomizer/releases).

=== "Install with curl"

    Install the latest release on macOS or Linux with:
    
    ```shell
    curl -s https://kustomizer.dev/install.sh | sudo bash
    ```
    
    If [cosign](https://github.com/sigstore/cosign) is found in PATH, the script will verify the signature
    of the release using the public key from [stefanprodan.keybase.pub/cosign/kustomizer.pub](https://stefanprodan.keybase.pub/cosign/kustomizer.pub).
    
    The install script does the following:

    - attempts to detect your OS
    - downloads the [release tar file](https://github.com/stefanprodan/kustomizer/releases) and its signature in a temporary directory
    - verifies the signature with cosign
    - unpacks the release tar file
    - verifies the binary checksum
    - copies the kustomizer binary to `/usr/local/bin`
    - removes the temporary directory

=== "Install from source"

    Using Go >= 1.17:
    
    ```shell
    go install github.com/stefanprodan/kustomizer/cmd/kustomizer@latest
    ```

## Configuration

In order to change settings such as the server-side apply field manager or the apply order,
first create a config file at `~/.kustomizer/config` with:

```console
$ kustomizer config init
config written to /Users/me/.kustomizer/config
```

Make adjustments to the config YAML, then validate the config with:

```console
$ kustomizer config view
apiVersion: kustomizer.dev/v1
kind: Config
applyOrder:
  first:
  - CustomResourceDefinition
  - Namespace
  - ResourceQuota
  - StorageClass
  - ServiceAccount
  - PodSecurityPolicy
  - Role
  - ClusterRole
  - RoleBinding
  - ClusterRoleBinding
  - ConfigMap
  - Secret
  - Service
  - LimitRange
  - PriorityClass
  - Deployment
  - StatefulSet
  - CronJob
  - PodDisruptionBudget
  last:
  - MutatingWebhookConfiguration
  - ValidatingWebhookConfiguration
fieldManager:
  group: inventory.kustomizer.dev
  name: kustomizer
```
