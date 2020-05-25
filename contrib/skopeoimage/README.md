<img src="https://cdn.rawgit.com/containers/skopeo/master/docs/skopeo.svg" width="250">

----

# skopeoimage

## Overview

This directory contains the Dockerfiles necessary to create the three skopeoimage container
images that are housed on quay.io under the skopeo account.  All three repositories where
the images live are public and can be pulled without credentials.  These container images
are secured and the resulting containers can run safely.  The container images are built
using the latest Fedora and then Skopeo is installed into them:

  * quay.io/skopeo/stable - This image is built using the latest stable version of Skopeo in a Fedora based container.  Built with skopeoimage/stable/Dockerfile.
  * quay.io/skopeo/upstream - This image is built using the latest code found in this GitHub repository.  When someone creates a commit and pushes it, the image is created.  Due to that the image changes frequently and is not guaranteed to be stable.  Built with skopeoimage/upstream/Dockerfile.
  * quay.io/skopeo/testing - This image is built using the latest version of Skopeo that is or was in updates testing for Fedora.  At times this may be the same as the stable image.  This container image will primarily be used by the development teams for verification testing when a new package is created.  Built with skopeoimage/testing/Dockerfile.

## Sample Usage

Although not required, it is suggested that [Podman](https://github.com/containers/libpod) be used with these container images.

```
# Get Help on Skopeo
podman run docker://quay.io/skopeo/stable:latest --help

# Get help on the Skopeo Copy command
podman run docker://quay.io/skopeo/stable:latest copy --help

# Copy the Skopeo container image from quay.io to
# a private registry
podman run docker://quay.io/skopeo/stable:latest copy docker://quay.io/skopeo/stable docker://registry.internal.company.com/skopeo

# Inspect the fedora:latest image
podman run docker://quay.io/skopeo/stable:latest inspect --config docker://registry.fedoraproject.org/fedora:latest  | jq
```
