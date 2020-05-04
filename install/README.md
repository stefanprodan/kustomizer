# Kustomizer Installation

Binaries for macOS and Linux AMD64 are available for download on the 
[release page](https://github.com/stefanprodan/kustomizer/releases).

To install the latest release run:

```bash
curl -s https://raw.githubusercontent.com/stefanprodan/kustomizer/master/install/kustomizer.sh | sudo bash
```

The install script does the following:
* attempts to detect your OS
* downloads and unpacks the release tar file in a temporary directory
* copies the kustomizer binary to `/usr/local/bin`
* removes the temporary directory

## Build from source

Clone the repository:

```bash
git clone https://github.com/stefanprodan/kustomizer
cd kustomizer
```

Build the kustomizer binary (requires go >= 1.14):

```bash
make build
```

Run the binary:

```bash
./bin/kustomizer -h
```
