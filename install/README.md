# Kustomizer Installation

To install the latest release on macOS or Linux:

```bash
curl -s https://raw.githubusercontent.com/stefanprodan/kustomizer/main/install/kustomizer.sh | sudo bash
```

The install script does the following:
* attempts to detect your OS
* downloads the [release tar file](https://github.com/stefanprodan/kustomizer/releases) and its signature in a temporary directory
* verifies the [cosign](https://github.com/sigstore/cosign) signature with [stefanprodan.keybase.pub](https://stefanprodan.keybase.pub/cosign/kustomizer.pub)
* unpacks the release tar file
* verifies the binary checksum
* copies the kustomizer binary to `/usr/local/bin`
* removes the temporary directory

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
