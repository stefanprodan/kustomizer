# Install

The Kustomizer CLI is available as a binary executable for all major platforms,
the binaries can be downloaded form GitHub [release page](https://github.com/stefanprodan/kustomizer/releases).

## Install with curl

Install the latest release on macOS or Linux with:

```bash
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

## Install from source

Using Go >= 1.17:

```sh
go install github.com/stefanprodan/kustomizer/cmd/kustomizer@latest
```

## Build from source

Clone the repository:

```bash
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

Build the kustomizer binary (requires go >= 1.17):

```bash
make build
```

Run the binary:

```bash
./bin/kustomizer -h
```
