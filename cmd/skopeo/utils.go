package main

import (
	"context"
	"errors"
	"strings"

	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/urfave/cli"
)

// imageOptions collects CLI flags which are the same across subcommands, but may be different for each image
// (e.g. may differ between the source and destination of a copy)
type imageOptions struct {
	global *globalOptions // May be shared across several imageOptions instances.
}

// imageFlags prepares a collection of CLI flags writing into imageOptions, and the managed imageOptions structure.
func imageFlags(global *globalOptions, flagPrefix string) ([]cli.Flag, *imageOptions) {
	opts := imageOptions{global: global}
	return []cli.Flag{}, &opts
}

func contextFromImageOptions(c *cli.Context, opts *imageOptions, flagPrefix string) (*types.SystemContext, error) {
	ctx := &types.SystemContext{
		RegistriesDirPath:                 opts.global.registriesDirPath,
		ArchitectureChoice:                opts.global.overrideArch,
		OSChoice:                          opts.global.overrideOS,
		DockerCertPath:                    c.String(flagPrefix + "cert-dir"),
		OSTreeTmpDirPath:                  c.String(flagPrefix + "ostree-tmp-dir"),
		OCISharedBlobDirPath:              c.String(flagPrefix + "shared-blob-dir"),
		DirForceCompress:                  c.Bool(flagPrefix + "compress"),
		AuthFilePath:                      c.String("authfile"),
		DockerDaemonHost:                  c.String(flagPrefix + "daemon-host"),
		DockerDaemonCertPath:              c.String(flagPrefix + "cert-dir"),
		DockerDaemonInsecureSkipTLSVerify: !c.BoolT(flagPrefix + "tls-verify"),
	}
	// DEPRECATED: We support this for backward compatibility, but override it if a per-image flag is provided.
	if opts.global.tlsVerify.present {
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!opts.global.tlsVerify.value)
	}
	if c.IsSet(flagPrefix + "tls-verify") {
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!c.BoolT(flagPrefix + "tls-verify"))
	}
	if c.IsSet(flagPrefix + "creds") {
		var err error
		ctx.DockerAuthConfig, err = getDockerAuth(c.String(flagPrefix + "creds"))
		if err != nil {
			return nil, err
		}
	}
	return ctx, nil
}

func parseCreds(creds string) (string, string, error) {
	if creds == "" {
		return "", "", errors.New("credentials can't be empty")
	}
	up := strings.SplitN(creds, ":", 2)
	if len(up) == 1 {
		return up[0], "", nil
	}
	if up[0] == "" {
		return "", "", errors.New("username can't be empty")
	}
	return up[0], up[1], nil
}

func getDockerAuth(creds string) (*types.DockerAuthConfig, error) {
	username, password, err := parseCreds(creds)
	if err != nil {
		return nil, err
	}
	return &types.DockerAuthConfig{
		Username: username,
		Password: password,
	}, nil
}

// parseImage converts image URL-like string to an initialized handler for that image.
// The caller must call .Close() on the returned ImageCloser.
func parseImage(ctx context.Context, c *cli.Context, opts *imageOptions) (types.ImageCloser, error) {
	imgName := c.Args().First()
	ref, err := alltransports.ParseImageName(imgName)
	if err != nil {
		return nil, err
	}
	sys, err := contextFromImageOptions(c, opts, "")
	if err != nil {
		return nil, err
	}
	return ref.NewImage(ctx, sys)
}

// parseImageSource converts image URL-like string to an ImageSource.
// The caller must call .Close() on the returned ImageSource.
func parseImageSource(ctx context.Context, c *cli.Context, opts *imageOptions, name string) (types.ImageSource, error) {
	ref, err := alltransports.ParseImageName(name)
	if err != nil {
		return nil, err
	}
	sys, err := contextFromImageOptions(c, opts, "")
	if err != nil {
		return nil, err
	}
	return ref.NewImageSource(ctx, sys)
}
