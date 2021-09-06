# Kustomizer Installation

To install the latest release on macOS or Linux:

```bash
curl -s https://raw.githubusercontent.com/stefanprodan/kustomizer/main/install/kustomizer.sh | sudo bash
```

The install script does the following:
* attempts to detect your OS
* downloads and unpacks the [release tar file](https://github.com/stefanprodan/kustomizer/releases) in a temporary directory
* verifies the binary checksum
* copies the kustomizer binary to `/usr/local/bin`
* removes the temporary directory

## Build from source

Clone the repository:

```bash
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

Build the kustomizer binary (requires go >= 1.16):

```bash
make build
```

Run the binary:

```bash
./bin/kustomizer -h
```
