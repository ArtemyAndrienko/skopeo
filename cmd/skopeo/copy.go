package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
)

// contextsFromCopyOptions returns source and destionation types.SystemContext depending on c.
func contextsFromCopyOptions(c *cli.Context, opts *copyOptions) (*types.SystemContext, *types.SystemContext, error) {
	sourceCtx, err := contextFromImageOptions(c, opts.srcImage)
	if err != nil {
		return nil, nil, err
	}

	destinationCtx, err := contextFromImageOptions(c, opts.destImage)
	if err != nil {
		return nil, nil, err
	}

	return sourceCtx, destinationCtx, nil
}

type copyOptions struct {
	global            *globalOptions
	srcImage          *imageOptions
	destImage         *imageOptions
	additionalTags    cli.StringSlice // For docker-archive: destinations, in addition to the name:tag specified as destination, also add these
	removeSignatures  bool            // Do not copy signatures from the source image
	signByFingerprint string          // Sign the image using a GPG key with the specified fingerprint
	format            optionalString  // Force conversion of the image to a specified format
}

func copyCmd(global *globalOptions) cli.Command {
	srcFlags, srcOpts := imageFlags(global, "src-", "screds")
	destFlags, destOpts := imageFlags(global, "dest-", "dcreds")
	opts := copyOptions{global: global,
		srcImage:  srcOpts,
		destImage: destOpts,
	}

	return cli.Command{
		Name:  "copy",
		Usage: "Copy an IMAGE-NAME from one location to another",
		Description: fmt.Sprintf(`

	Container "IMAGE-NAME" uses a "transport":"details" format.

	Supported transports:
	%s

	See skopeo(1) section "IMAGE NAMES" for the expected format
	`, strings.Join(transports.ListNames(), ", ")),
		ArgsUsage: "SOURCE-IMAGE DESTINATION-IMAGE",
		Action:    opts.run,
		// FIXME: Do we need to namespace the GPG aspect?
		Flags: append(append([]cli.Flag{
			cli.StringSliceFlag{
				Name:  "additional-tag",
				Usage: "additional tags (supports docker-archive)",
				Value: &opts.additionalTags, // Surprisingly StringSliceFlag does not support Destination:, but modifies Value: in place.
			},
			cli.StringFlag{
				Name:  "authfile",
				Usage: "path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
			},
			cli.BoolFlag{
				Name:        "remove-signatures",
				Usage:       "Do not copy signatures from SOURCE-IMAGE",
				Destination: &opts.removeSignatures,
			},
			cli.StringFlag{
				Name:        "sign-by",
				Usage:       "Sign the image using a GPG key with the specified `FINGERPRINT`",
				Destination: &opts.signByFingerprint,
			},
			cli.StringFlag{
				Name:  "dest-ostree-tmp-dir",
				Value: "",
				Usage: "`DIRECTORY` to use for OSTree temporary files",
			},
			cli.StringFlag{
				Name:  "src-shared-blob-dir",
				Value: "",
				Usage: "`DIRECTORY` to use to fetch retrieved blobs (OCI layout sources only)",
			},
			cli.StringFlag{
				Name:  "dest-shared-blob-dir",
				Value: "",
				Usage: "`DIRECTORY` to use to store retrieved blobs (OCI layout destinations only)",
			},
			cli.GenericFlag{
				Name:  "format, f",
				Usage: "`MANIFEST TYPE` (oci, v2s1, or v2s2) to use when saving image to directory using the 'dir:' transport (default is manifest type of source)",
				Value: newOptionalStringValue(&opts.format),
			},
			cli.BoolFlag{
				Name:  "dest-compress",
				Usage: "Compress tarball image layers when saving to directory using the 'dir' transport. (default is same compression type as source)",
			},
			cli.StringFlag{
				Name:  "src-daemon-host",
				Value: "",
				Usage: "use docker daemon host at `HOST` (docker-daemon sources only)",
			},
			cli.StringFlag{
				Name:  "dest-daemon-host",
				Value: "",
				Usage: "use docker daemon host at `HOST` (docker-daemon destinations only)",
			},
		}, srcFlags...), destFlags...),
	}
}

func (opts *copyOptions) run(c *cli.Context) error {
	if len(c.Args()) != 2 {
		cli.ShowCommandHelp(c, "copy")
		return errors.New("Exactly two arguments expected")
	}

	policyContext, err := opts.global.getPolicyContext()
	if err != nil {
		return fmt.Errorf("Error loading trust policy: %v", err)
	}
	defer policyContext.Destroy()

	srcRef, err := alltransports.ParseImageName(c.Args()[0])
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", c.Args()[0], err)
	}
	destRef, err := alltransports.ParseImageName(c.Args()[1])
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", c.Args()[1], err)
	}

	sourceCtx, destinationCtx, err := contextsFromCopyOptions(c, opts)
	if err != nil {
		return err
	}

	var manifestType string
	if opts.format.present {
		switch opts.format.value {
		case "oci":
			manifestType = imgspecv1.MediaTypeImageManifest
		case "v2s1":
			manifestType = manifest.DockerV2Schema1SignedMediaType
		case "v2s2":
			manifestType = manifest.DockerV2Schema2MediaType
		default:
			return fmt.Errorf("unknown format %q. Choose one of the supported formats: 'oci', 'v2s1', or 'v2s2'", opts.format.value)
		}
	}

	for _, image := range opts.additionalTags {
		ref, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			return fmt.Errorf("error parsing additional-tag '%s': %v", image, err)
		}
		namedTagged, isNamedTagged := ref.(reference.NamedTagged)
		if !isNamedTagged {
			return fmt.Errorf("additional-tag '%s' must be a tagged reference", image)
		}
		destinationCtx.DockerArchiveAdditionalTags = append(destinationCtx.DockerArchiveAdditionalTags, namedTagged)
	}

	ctx, cancel := opts.global.commandTimeoutContext()
	defer cancel()

	_, err = copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{
		RemoveSignatures:      opts.removeSignatures,
		SignBy:                opts.signByFingerprint,
		ReportWriter:          os.Stdout,
		SourceCtx:             sourceCtx,
		DestinationCtx:        destinationCtx,
		ForceManifestMIMEType: manifestType,
	})
	return err
}
