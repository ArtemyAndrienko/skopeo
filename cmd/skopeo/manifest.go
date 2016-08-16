package main

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/containers/image/manifest"
	"github.com/urfave/cli"
)

func manifestDigest(context *cli.Context) error {
	if len(context.Args()) != 1 {
		return errors.New("Usage: skopeo manifest-digest manifest")
	}
	manifestPath := context.Args()[0]

	man, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("Error reading manifest from %s: %v", manifestPath, err)
	}
	digest, err := manifest.Digest(man)
	if err != nil {
		return fmt.Errorf("Error computing digest: %v", err)
	}
	fmt.Fprintf(context.App.Writer, "%s\n", digest)
	return nil
}

var manifestDigestCmd = cli.Command{
	Name:      "manifest-digest",
	Usage:     "Compute a manifest digest of a file",
	ArgsUsage: "MANIFEST",
	Action:    manifestDigest,
}
