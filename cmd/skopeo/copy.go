package main

import (
	"errors"
	"fmt"

	"github.com/containers/image/copy"
	"github.com/containers/image/transports"
	"github.com/urfave/cli"
)

func copyHandler(context *cli.Context) error {
	if len(context.Args()) != 2 {
		return errors.New("Usage: copy source destination")
	}

	policyContext, err := getPolicyContext(context)
	if err != nil {
		return fmt.Errorf("Error loading verification policy: %v", err)
	}
	defer policyContext.Destroy()

	srcRef, err := transports.ParseImageName(context.Args()[0])
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", context.Args()[0], err)
	}
	destRef, err := transports.ParseImageName(context.Args()[1])
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", context.Args()[1], err)
	}
	signBy := context.String("sign-by")

	return copy.Image(contextFromGlobalOptions(context), policyContext, destRef, srcRef, &copy.Options{
		SignBy: signBy,
	})
}

var copyCmd = cli.Command{
	Name:      "copy",
	Usage:     "Copy an image from one location to another",
	ArgsUsage: "SOURCE-IMAGE DESTINATION-IMAGE",
	Action:    copyHandler,
	// FIXME: Do we need to namespace the GPG aspect?
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "sign-by",
			Usage: "Sign the image using a GPG key with the specified `FINGERPRINT`",
		},
	},
}
