Naisd
=====

[![Build Status](https://travis-ci.org/nais/naisd.svg?branch=master)](https://travis-ci.org/nais/naisd)
[![Go Report Card](https://goreportcard.com/badge/github.com/nais/naisd)](https://goreportcard.com/report/github.com/nais/naisd)


k8s in-cluster daemon with API for performing NAIS-operations


## Basic outline

1. HTTP POST to API with name of application, version and environment
2. Fetches manifest from internal artifact repository
3. Extract info from yaml
4. Get and inject environment specific variables from Fasit
5. Creates appropriate k8s resources


## nais cli

The `nais` cli will help you in validating your `nais.yaml`, uploading it to Nexus and deploying your application. Very useful for your CI/CD servers.


### Basic Usage

#### Validating

```sh
nais validate [flags]

Flags:
  -f, --file string   path to manifest (default "nais.yaml")
  -o, --output        prints full manifest including defaults
```

Will validate `nais.yaml` by default. Specify another file using the `-f` or `--file` argument.

Will exit with status `0` on success, `1` on failure.


#### Uploading

```
nais upload [flags]

Flags:
  -a, --app string        name of your app
  -f, --file string       path to nais.yaml (default "nais.yaml")
  -p, --password string   the password
  -u, --username string   the username
  -v, --version string    version you want to upload
```

Will upload `nais.yaml` to Nexus.

The username and password may be specified using environment variable `NEXUS_USERNAME` and `NEXUS_PASSWORD`, and
the Nexus repo can be specified using `NEXUS_URL`.

**Example:** Uploading to Nexus 2:

```
NEXUS_URL=http://maven.adeo.no/nexus/content/repositories/m2internal ./nais upload --app myapp -v 10
```

**Example:** Uploading to Nexus 3:

```
./nais upload --app myapp -v 10
```


#### Deploy

```sh
nais deploy [flags]

Flags:
  -a, --app string            name of your app
  -c, --cluster string        the cluster you want to deploy to (default: "preprod-fss")
  -e, --environment string    environment you want to use (default "q0")
  -m, --manifest-url string   alternative URL to the nais manifest
  -n, --namespace string      the kubernetes namespace (default "default")
  -p, --fasit-password string the password
  -u, --fasit-username string the username
  -v, --version string        version you want to deploy
      --wait                  whether to wait until the deploy has succeeded (or failed)
  -z, --zone string           the zone the app will be in (default "fss")
```

If using default values, only `app`, `version`, `fasit-username` and `fasit-password` is required.

The username and password may be specified using environment variable `FASIT_USERNAME` and `FASIT_PASSWORD` instead.


### Installation

Binaries for `amd64` Linux, Darwin and Windows are automatically released on every build.

The commands below will assume you have already [downloaded a release](https://github.com/nais/naisd/releases).


### Install Linux/macOS

```sh
xz -d nais-<arch>-amd64.xz
mv nais-<arch>-amd64 /usr/local/bin/nais
chmod +x /usr/local/bin/nais
```

Where `<arch>` will be `linux` or `darwin`.


### Windows

Unzip the release and place it somewhere.


## CI

on push:

- run tests
- produce binary
- bump version
- make and publish alpine docker image with binary to dockerhub
- make and publish corresponding helm chart to quay.io 


## dev notes

For local development, use minikube. You can run naisd.go with -kubeconfig=<path to kube config> for testing without deploying to cluster. 

```dep ensure```

...to fetch dependecies

To reduce build time, do

```go build -i .```

initially. 
