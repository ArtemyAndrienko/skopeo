package main

import (
	"io/ioutil"
	"strings"

	"github.com/containers/image/directory"
	"github.com/containers/image/image"
	"github.com/urfave/cli"
)

// TODO(runcom): document args and usage
var layersCmd = cli.Command{
	Name:  "layers",
	Usage: "get images layers",
	Action: func(c *cli.Context) error {
		rawSource, err := parseImageSource(c, c.Args()[0])
		if err != nil {
			return err
		}
		src := image.FromSource(rawSource)
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
		dest := directory.NewDirImageDestination(tmpDir)
		manifest, _, err := src.Manifest()
		if err != nil {
			return err
		}
		if err := dest.PutManifest(manifest); err != nil {
			return err
		}
		for _, digest := range blobDigests {
			if !strings.HasPrefix(digest, "sha256:") {
				digest = "sha256:" + digest
			}
			r, _, err := rawSource.GetBlob(digest)
			if err != nil {
				return err
			}
			if err := dest.PutBlob(digest, r); err != nil {
				r.Close()
				return err
			}
			r.Close()
		}
		return nil
	},
}
