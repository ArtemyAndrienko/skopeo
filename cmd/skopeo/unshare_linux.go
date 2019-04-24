package main

import (
	"github.com/containers/buildah/pkg/unshare"
	"github.com/pkg/errors"
	"github.com/syndtr/gocapability/capability"
)

var neededCapabilities = []capability.Cap{
	capability.CAP_CHOWN,
	capability.CAP_DAC_OVERRIDE,
	capability.CAP_FOWNER,
	capability.CAP_FSETID,
	capability.CAP_MKNOD,
	capability.CAP_SETFCAP,
}

func maybeReexec() error {
	// With Skopeo we need only the subset of the root capabilities necessary
	// for pulling an image to the storage.  Do not attempt to create a namespace
	// if we already have the capabilities we need.
	capabilities, err := capability.NewPid(0)
	if err != nil {
		return errors.Wrapf(err, "error reading the current capabilities sets")
	}
	for _, cap := range neededCapabilities {
		if !capabilities.Get(capability.EFFECTIVE, cap) {
			// We miss a capability we need, create a user namespaces
			unshare.MaybeReexecUsingUserNamespace(true)
			return nil
		}
	}
	return nil
}
