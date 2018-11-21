package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"strings"

	"io"
)

// tagListOutput is the output format of (skopeo list-tags), primarily so that we can format it with a simple json.MarshalIndent.
type tagListOutput struct {
	Repository string
	Tags       []string
}

type tagsOptions struct {
	global *globalOptions
	image  *imageOptions
}

func tagsCmd(global *globalOptions) cli.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	imageFlags, imageOpts := dockerImageFlags(global, sharedOpts, "", "")

	opts := tagsOptions{
		global: global,
		image:  imageOpts,
	}

	return cli.Command{
		Name:  "list-tags",
		Usage: "List tags in the transport/repository specified by the REPOSITORY-NAME",
		Description: `
	Return the list of tags from the transport/repository "REPOSITORY-NAME"
	
    Supported transports:
	docker

	See skopeo-list-tags(1) section "REPOSITORY NAMES" for the expected format
	`,
		ArgsUsage: "REPOSITORY-NAME",
		Flags:     append(sharedFlags, imageFlags...),
		Action:    commandAction(opts.run),
	}
}

// Customized version of the alltransports.ParseImageName and docker.ParseReference that does not place a default tag in the reference
// Would really love to not have this, but needed to enforce tag-less and digest-less names
func parseDockerRepositoryReference(refString string) (types.ImageReference, error) {
	parts := strings.SplitN(refString, ":", 2)
	if len(parts) != 2 {
		return nil, errors.Errorf(`Invalid image name "%s", expected colon-separated transport:reference`, refString)
	}

	transport := transports.Get(parts[0])
	if transport == nil || transport.Name() != docker.Transport.Name() {
		return nil, errors.New("Invalid transport, can only parse docker transport references")
	}

	if !strings.HasPrefix(parts[1], "//") {
		return nil, errors.Errorf("docker: image reference %s does not start with //", parts[1])
	}

	ref, err := reference.ParseNormalizedNamed(strings.TrimPrefix(parts[1], "//"))
	if err != nil {
		return nil, err
	}

	if !reference.IsNameOnly(ref) {
		return nil, errors.New(`No tag or digest allowed in reference`)
	}

	// Checks ok, now return a reference. This is a hack because the tag listing code expects a full image reference even though the tag is ignored
	return docker.NewReference(reference.TagNameOnly(ref))
}

func listDockerTags(ctx context.Context, sys *types.SystemContext, referenceInput string) (string, []string, error) {
	imgRef, err := parseDockerRepositoryReference(referenceInput)
	if err != nil {
		return ``, nil, fmt.Errorf("Cannot parse repository reference: %v", err)
	}

	repositoryName := imgRef.DockerReference().Name()

	tags, err := docker.GetRepositoryTags(ctx, sys, imgRef)
	if err != nil {
		return ``, nil, fmt.Errorf("Error listing repository tags: %v", err)
	}
	return repositoryName, tags, nil
}

func (opts *tagsOptions) run(args []string, stdout io.Writer) (retErr error) {
	ctx, cancel := opts.global.commandTimeoutContext()
	defer cancel()
	var repositoryName string
	var tagListing []string

	if len(args) != 1 {
		return errorShouldDisplayUsage{errors.New("Exactly one non-option argument expected")}
	}

	sys, err := opts.image.newSystemContext()
	if err != nil {
		return err
	}

	transport := alltransports.TransportFromImageName(args[0])
	if transport == nil {
		return errors.New("Invalid transport")
	}

	if transport.Name() == docker.Transport.Name() {
		repositoryName, tagListing, err = listDockerTags(ctx, sys, args[0])
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("Unsupported transport '%v' for tag listing. Only '%v' currently supported", transport.Name(), docker.Transport.Name())
	}

	outputData := tagListOutput{
		Repository: repositoryName,
		Tags:       tagListing,
	}

	out, err := json.MarshalIndent(outputData, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s\n", string(out))

	return err
}
