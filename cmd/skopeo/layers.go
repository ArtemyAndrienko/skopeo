package main

import (
	"errors"
	"io/ioutil"
	"strings"

	"github.com/containers/image/directory"
	"github.com/containers/image/image"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/urfave/cli"
)

// TODO(runcom): document args and usage
var layersCmd = cli.Command{
	Name:      "layers",
	Usage:     "Get layers of IMAGE-NAME",
	ArgsUsage: "IMAGE-NAME [LAYER...]",
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return errors.New("please specify an image")
		}
		rawSource, err := parseImageSource(c, c.Args()[0], []string{
			// TODO: skopeo layers only supports these now
			// eventually we'll remove this command altogether...
			manifest.DockerV2Schema1SignedMediaType,
			manifest.DockerV2Schema1MediaType,
		})
		if err != nil {
			return err
		}
		src, err := image.FromSource(rawSource)
		if err != nil {
			rawSource.Close()
			return err
		}
		defer src.Close()

		blobDigests := c.Args().Tail()
		if len(blobDigests) == 0 {
			layers := src.LayerInfos()
			seenLayers := map[string]struct{}{}
			for _, info := range layers {
				if _, ok := seenLayers[info.Digest]; !ok {
					blobDigests = append(blobDigests, info.Digest)
					seenLayers[info.Digest] = struct{}{}
				}
			}
			configInfo := src.ConfigInfo()
			if configInfo.Digest != "" {
				blobDigests = append(blobDigests, configInfo.Digest)
			}
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
			if _, err := dest.PutBlob(r, types.BlobInfo{Digest: digest, Size: blobSize}); err != nil {
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
