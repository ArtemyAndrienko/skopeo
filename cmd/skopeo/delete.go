package main

import (
	"errors"
	"fmt"

	"github.com/containers/image/transports"
	"github.com/urfave/cli"
)

func deleteHandler(context *cli.Context) error {
	if len(context.Args()) != 1 {
		return errors.New("Usage: delete imageReference")
	}

	ref, err := transports.ParseImageName(context.Args()[0])
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", context.Args()[0], err)
	}

	if err := ref.DeleteImage(contextFromGlobalOptions(context)); err != nil {
		return err
	}
	return nil
}

var deleteCmd = cli.Command{
	Name:      "delete",
	Usage:     "Delete image IMAGE-NAME",
	ArgsUsage: "IMAGE-NAME",
	Action:    deleteHandler,
}
