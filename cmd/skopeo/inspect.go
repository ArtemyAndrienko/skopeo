package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/containers/image/docker"
	"github.com/containers/image/manifest"
	"github.com/containers/image/transports"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// inspectOutput is the output format of (skopeo inspect), primarily so that we can format it with a simple json.MarshalIndent.
type inspectOutput struct {
	Name          string `json:",omitempty"`
	Tag           string `json:",omitempty"`
	Digest        digest.Digest
	RepoTags      []string
	Created       *time.Time
	DockerVersion string
	Labels        map[string]string
	Architecture  string
	Os            string
	Layers        []string
}

type inspectOptions struct {
	global *globalOptions
	image  *imageOptions
	raw    bool // Output the raw manifest instead of parsing information about the image
}

func inspectCmd(global *globalOptions) cli.Command {
	imageFlags, imageOpts := imageFlags(global, "")
	opts := inspectOptions{
		global: global,
		image:  imageOpts,
	}
	return cli.Command{
		Name:  "inspect",
		Usage: "Inspect image IMAGE-NAME",
		Description: fmt.Sprintf(`
	Return low-level information about "IMAGE-NAME" in a registry/transport

	Supported transports:
	%s

	See skopeo(1) section "IMAGE NAMES" for the expected format
	`, strings.Join(transports.ListNames(), ", ")),
		ArgsUsage: "IMAGE-NAME",
		Flags: append([]cli.Flag{
			cli.StringFlag{
				Name:  "authfile",
				Usage: "path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json",
			},
			cli.StringFlag{
				Name:  "cert-dir",
				Value: "",
				Usage: "use certificates at `PATH` (*.crt, *.cert, *.key) to connect to the registry",
			},
			cli.BoolTFlag{
				Name:  "tls-verify",
				Usage: "require HTTPS and verify certificates when talking to container registries (defaults to true)",
			},
			cli.BoolFlag{
				Name:        "raw",
				Usage:       "output raw manifest",
				Destination: &opts.raw,
			},
			cli.StringFlag{
				Name:  "creds",
				Value: "",
				Usage: "Use `USERNAME[:PASSWORD]` for accessing the registry",
			},
		}, imageFlags...),
		Action: opts.run,
	}
}

func (opts *inspectOptions) run(c *cli.Context) (retErr error) {
	ctx, cancel := opts.global.commandTimeoutContext()
	defer cancel()

	img, err := parseImage(ctx, c, opts.image)
	if err != nil {
		return err
	}

	defer func() {
		if err := img.Close(); err != nil {
			retErr = errors.Wrapf(retErr, fmt.Sprintf("(could not close image: %v) ", err))
		}
	}()

	rawManifest, _, err := img.Manifest(ctx)
	if err != nil {
		return err
	}
	if opts.raw {
		_, err := c.App.Writer.Write(rawManifest)
		if err != nil {
			return fmt.Errorf("Error writing manifest to standard output: %v", err)
		}
		return nil
	}
	imgInspect, err := img.Inspect(ctx)
	if err != nil {
		return err
	}
	outputData := inspectOutput{
		Name: "", // Set below if DockerReference() is known
		Tag:  imgInspect.Tag,
		// Digest is set below.
		RepoTags:      []string{}, // Possibly overriden for docker.Transport.
		Created:       imgInspect.Created,
		DockerVersion: imgInspect.DockerVersion,
		Labels:        imgInspect.Labels,
		Architecture:  imgInspect.Architecture,
		Os:            imgInspect.Os,
		Layers:        imgInspect.Layers,
	}
	outputData.Digest, err = manifest.Digest(rawManifest)
	if err != nil {
		return fmt.Errorf("Error computing manifest digest: %v", err)
	}
	if dockerRef := img.Reference().DockerReference(); dockerRef != nil {
		outputData.Name = dockerRef.Name()
	}
	if img.Reference().Transport() == docker.Transport {
		sys, err := contextFromImageOptions(c, opts.image)
		if err != nil {
			return err
		}
		outputData.RepoTags, err = docker.GetRepositoryTags(ctx, sys, img.Reference())
		if err != nil {
			// some registries may decide to block the "list all tags" endpoint
			// gracefully allow the inspect to continue in this case. Currently
			// the IBM Bluemix container registry has this restriction.
			if !strings.Contains(err.Error(), "401") {
				return fmt.Errorf("Error determining repository tags: %v", err)
			}
			logrus.Warnf("Registry disallows tag list retrieval; skipping")
		}
	}
	out, err := json.MarshalIndent(outputData, "", "    ")
	if err != nil {
		return err
	}
	fmt.Fprintln(c.App.Writer, string(out))
	return nil
}
