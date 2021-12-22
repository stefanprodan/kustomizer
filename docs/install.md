# Install

The Kustomizer CLI is available as a binary executable for all major platforms,
the binaries can be downloaded form GitHub [release page](https://github.com/stefanprodan/kustomizer/releases).

=== "Install with brew"

    Install the latest release on macOS or Linux with:
    
    ```shell
    brew install stefanprodan/tap/kustomizer
    ```

    Note that the Homebrew formula will setup shell autocompletion for Bash, Fish and ZSH.

=== "Install with curl"

    Install the latest release on macOS or Linux with:
    
    ```shell
    curl -s https://kustomizer.dev/install.sh | sudo bash
    ```

    The install script downloads the latest release from GitHub and copies the kustomizer binary to `/usr/local/bin`.
    If [cosign](https://github.com/sigstore/cosign) is found in PATH, the script will verify the signature
    of the release using the public key from [stefanprodan.keybase.pub/cosign/kustomizer.pub](https://stefanprodan.keybase.pub/cosign/kustomizer.pub).

=== "Install from source"

    Using Go >= 1.17:
    
    ```shell
    go install github.com/stefanprodan/kustomizer/cmd/kustomizer@latest
    ```

## Shell autocompletion

Configure your shell to load kustomizer completions:

=== "bash"

    To load completion run:
    
    ```shell
    . <(kustomizer completion bash)
    ```

    To configure your bash shell to load completions for each session add to your bashrc:

    ```shell
    # ~/.bashrc or ~/.profile
    command -v kustomizer >/dev/null && . <(kustomizer completion bash)
    ```

    If you have an alias for kustomizer, you can extend shell completion to work with that alias:

    ```shell
    echo 'alias kz=kustomizer' >>~/.bashrc
    echo 'complete -F __start_kustomizer kz' >>~/.bashrc
    ```

=== "fish"

    To configure your fish shell to [load completions](http://fishshell.com/docs/current/index.html#completion-own)
    for each session write this script to your completions dir:
    
    ```shell
    kustomizer completion fish > ~/.config/fish/completions/kustomizer.fish
    ```

=== "powershell"

    To load completion run:

    ```shell
    . <(kustomizer completion powershell)
    ```

    To configure your powershell shell to load completions for each session add to your powershell profile:
    
    Windows:

    ```shell
    cd "$env:USERPROFILE\Documents\WindowsPowerShell\Modules"
    kustomizer completion >> kustomizer-completion.ps1
    ```
    Linux:

    ```shell
    cd "${XDG_CONFIG_HOME:-"$HOME/.config/"}/powershell/modules"
    kustomizer completion >> kustomizer-completions.ps1
    ```

=== "zsh"

    To load completion run:
    
    ```shell
    . <(kustomizer completion zsh) && compdef _kustomizer kustomizer
    ```

    To configure your zsh shell to load completions for each session add to your zshrc:
    
    ```shell
    # ~/.zshrc or ~/.profile
    command -v kustomizer >/dev/null && . <(kustomizer completion zsh) && compdef _kustomizer kustomizer
    ```

    or write a cached file in one of the completion directories in your ${fpath}:
    
    ```shell
    echo "${fpath// /\n}" | grep -i completion
    kustomizer completion zsh > _kustomizer
    
    mv _kustomizer ~/.oh-my-zsh/completions  # oh-my-zsh
    mv _kustomizer ~/.zprezto/modules/completion/external/src/  # zprezto
    ```

## Configuration

In order to change settings such as the server-side apply field manager or the apply order,
first create a config file at `~/.kustomizer/config` with:

=== "command"

    ```shell
    kustomizer config init
    ```

=== "example output"

    ```console
    config written to /Users/stefanprodan/.kustomizer/config
    ```

Make adjustments to the config YAML, then validate the config with:

=== "command"
    
    ```shell
    kustomizer config view
    ```

=== "example output"

    ```yaml
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

If you want to use Kustomizer as a debug tool for Flux, you can set the field manager
to match Flux's [kustomize-controller](https://github.com/fluxcd/kustomize-controller) with:

```yaml
apiVersion: kustomizer.dev/v1
kind: Config
fieldManager:
  group: kustomize.toolkit.fluxcd.io
  name: kustomize-controller
```
