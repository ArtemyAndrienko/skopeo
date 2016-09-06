package main

import (
	"io/ioutil"
	"strings"

	"github.com/containers/image/directory"
	"github.com/containers/image/image"
	"github.com/containers/image/manifest"
	"github.com/urfave/cli"
)

// TODO(runcom): document args and usage
var layersCmd = cli.Command{
	Name:      "layers",
	Usage:     "Get layers of IMAGE-NAME",
	ArgsUsage: "IMAGE-NAME",
	Action: func(c *cli.Context) error {
		rawSource, err := parseImageSource(c, c.Args()[0], []string{
			// TODO: skopeo layers only support these now
			// eventually we'll remove this command altogether...
			manifest.DockerV2Schema1SignedMIMEType,
			manifest.DockerV2Schema1MIMEType,
		})
		if err != nil {
			return err
		}
		src := image.FromSource(rawSource)
		defer src.Close()

		blobDigests := c.Args().Tail()
		if len(blobDigests) == 0 {
			b, err := src.BlobDigests()
			if err != nil {
				return err
			}
			blobDigests = b
		}
		tmpDir, err := ioutil.TempDir(".", "layers-")
		if err != nil {
			return err
		}
		tmpDirRef, err := directory.NewReference(tmpDir)
		if err != nil {
			return err
		}
		dest, err := tmpDirRef.NewImageDestination(nil)
		if err != nil {
			return err
		}
		defer dest.Close()

		for _, digest := range blobDigests {
			if !strings.HasPrefix(digest, "sha256:") {
				digest = "sha256:" + digest
			}
			r, blobSize, err := rawSource.GetBlob(digest)
			if err != nil {
				return err
			}
			if _, _, err := dest.PutBlob(r, digest, blobSize); err != nil {
				r.Close()
				return err
			}
			r.Close()
		}

		manifest, _, err := src.Manifest()
		if err != nil {
			return err
		}
		if err := dest.PutManifest(manifest); err != nil {
			return err
		}

		if err := dest.Commit(); err != nil {
			return err
		}

		return nil
	},
}
