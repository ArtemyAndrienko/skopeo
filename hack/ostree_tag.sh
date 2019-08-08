#!/bin/bash

if test $(${GO:-go} env GOOS) != "linux" ; then
	exit 0
fi

if pkg-config ostree-1 &> /dev/null ; then
	# ostree: used by containers/storage
	# containers_image_ostree: used by containers/image
	echo "ostree containers_image_ostree"
fi
