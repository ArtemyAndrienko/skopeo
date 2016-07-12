package main

import (
	"errors"
	"fmt"

	"github.com/containers/image/image"
	"github.com/containers/image/signature"
	"github.com/urfave/cli"
)

func copyHandler(context *cli.Context) error {
	if len(context.Args()) != 2 {
		return errors.New("Usage: copy source destination")
	}

	dest, err := parseImageDestination(context, context.Args()[1])
	if err != nil {
		return fmt.Errorf("Error initializing %s: %v", context.Args()[1], err)
	}

	rawSource, err := parseImageSource(context, context.Args()[0])
	if err != nil {
		return fmt.Errorf("Error initializing %s: %v", context.Args()[0], err)
	}
	src := image.FromSource(rawSource, dest.SupportedManifestMIMETypes())

	signBy := context.String("sign-by")

	manifest, _, err := src.Manifest()
	if err != nil {
		return fmt.Errorf("Error reading manifest: %v", err)
	}

	blobDigests, err := src.BlobDigests()
	if err != nil {
		return fmt.Errorf("Error parsing manifest: %v", err)
	}
	for _, digest := range blobDigests {
		// TODO(mitr): do not ignore the size param returned here
		stream, _, err := rawSource.GetBlob(digest)
		if err != nil {
			return fmt.Errorf("Error reading blob %s: %v", digest, err)
		}
		defer stream.Close()
		if err := dest.PutBlob(digest, stream); err != nil {
			return fmt.Errorf("Error writing blob: %v", err)
		}
	}

	sigs, err := src.Signatures()
	if err != nil {
		return fmt.Errorf("Error reading signatures: %v", err)
	}

	if signBy != "" {
		mech, err := signature.NewGPGSigningMechanism()
		if err != nil {
			return fmt.Errorf("Error initializing GPG: %v", err)
		}
		dockerReference := dest.CanonicalDockerReference()
		if dockerReference == nil {
			return errors.New("Destination image does not have an associated Docker reference")
		}

		newSig, err := signature.SignDockerManifest(manifest, dockerReference.String(), mech, signBy)
		if err != nil {
			return fmt.Errorf("Error creating signature: %v", err)
		}
		sigs = append(sigs, newSig)
	}

	if err := dest.PutSignatures(sigs); err != nil {
		return fmt.Errorf("Error writing signatures: %v", err)
	}

	// FIXME: We need to call PutManifest after PutBlob and PutSignatures. This seems ugly; move to a "set properties" + "commit" model?
	if err := dest.PutManifest(manifest); err != nil {
		return fmt.Errorf("Error writing manifest: %v", err)
	}
	return nil
}

var copyCmd = cli.Command{
	Name:   "copy",
	Action: copyHandler,
	// FIXME: Do we need to namespace the GPG aspect?
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "sign-by",
			Usage: "sign the image using a GPG key with the specified fingerprint",
		},
	},
}
