package main

import (
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/urfave/cli"
)

// ParseImage converts image URL-like string to an initialized handler for that image.
func parseImage(c *cli.Context) (types.Image, error) {
	var (
		imgName   = c.Args().First()
		certPath  = c.GlobalString("cert-path")
		tlsVerify = c.GlobalBool("tls-verify")
	)
	ref, err := transports.ParseImageName(imgName)
	if err != nil {
		return nil, err
	}
	return ref.NewImage(certPath, tlsVerify)
}

// parseImageSource converts image URL-like string to an ImageSource.
func parseImageSource(c *cli.Context, name string) (types.ImageSource, error) {
	var (
		certPath  = c.GlobalString("cert-path")
		tlsVerify = c.GlobalBool("tls-verify") // FIXME!! defaults to false?
	)
	ref, err := transports.ParseImageName(name)
	if err != nil {
		return nil, err
	}
	return ref.NewImageSource(certPath, tlsVerify)
}

// parseImageDestination converts image URL-like string to an ImageDestination.
func parseImageDestination(c *cli.Context, name string) (types.ImageDestination, error) {
	var (
		certPath  = c.GlobalString("cert-path")
		tlsVerify = c.GlobalBool("tls-verify") // FIXME!! defaults to false?
	)
	ref, err := transports.ParseImageName(name)
	if err != nil {
		return nil, err
	}
	return ref.NewImageDestination(certPath, tlsVerify)
}
