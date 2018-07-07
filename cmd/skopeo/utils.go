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
	global         *globalOptions // May be shared across several imageOptions instances.
	flagPrefix     string         // FIXME: Drop this eventually.
	credsOption    optionalString // username[:password] for accessing a registry
	dockerCertPath string         // A directory using Docker-like *.{crt,cert,key} files for connecting to a registry or a daemon
	tlsVerify      optionalBool   // Require HTTPS and verify certificates (for docker: and docker-daemon:)
}

// imageFlags prepares a collection of CLI flags writing into imageOptions, and the managed imageOptions structure.
func imageFlags(global *globalOptions, flagPrefix, credsOptionAlias string) ([]cli.Flag, *imageOptions) {
	opts := imageOptions{
		global:     global,
		flagPrefix: flagPrefix,
	}

	// This is horribly ugly, but we need to support the old option forms of (skopeo copy) for compatibility.
	// Don't add any more cases like this.
	credsOptionExtra := ""
	if credsOptionAlias != "" {
		credsOptionExtra += "," + credsOptionAlias
	}

	return []cli.Flag{
		cli.GenericFlag{
			Name:  flagPrefix + "creds" + credsOptionExtra,
			Usage: "Use `USERNAME[:PASSWORD]` for accessing the registry",
			Value: newOptionalStringValue(&opts.credsOption),
		},
		cli.StringFlag{
			Name:        flagPrefix + "cert-dir",
			Usage:       "use certificates at `PATH` (*.crt, *.cert, *.key) to connect to the registry or daemon",
			Destination: &opts.dockerCertPath,
		},
		cli.GenericFlag{
			Name:  flagPrefix + "tls-verify",
			Usage: "require HTTPS and verify certificates when talking to the container registry or daemon (defaults to true)",
			Value: newOptionalBoolValue(&opts.tlsVerify),
		},
	}, &opts
}

func contextFromImageOptions(c *cli.Context, opts *imageOptions) (*types.SystemContext, error) {
	ctx := &types.SystemContext{
		RegistriesDirPath:    opts.global.registriesDirPath,
		ArchitectureChoice:   opts.global.overrideArch,
		OSChoice:             opts.global.overrideOS,
		DockerCertPath:       opts.dockerCertPath,
		OSTreeTmpDirPath:     c.String(opts.flagPrefix + "ostree-tmp-dir"),
		OCISharedBlobDirPath: c.String(opts.flagPrefix + "shared-blob-dir"),
		DirForceCompress:     c.Bool(opts.flagPrefix + "compress"),
		AuthFilePath:         c.String("authfile"),
		DockerDaemonHost:     c.String(opts.flagPrefix + "daemon-host"),
		DockerDaemonCertPath: opts.dockerCertPath,
	}
	if opts.tlsVerify.present {
		ctx.DockerDaemonInsecureSkipTLSVerify = !opts.tlsVerify.value
	}
	// DEPRECATED: We support this for backward compatibility, but override it if a per-image flag is provided.
	if opts.global.tlsVerify.present {
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!opts.global.tlsVerify.value)
	}
	if opts.tlsVerify.present {
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!opts.tlsVerify.value)
	}
	if opts.credsOption.present {
		var err error
		ctx.DockerAuthConfig, err = getDockerAuth(opts.credsOption.value)
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
	sys, err := contextFromImageOptions(c, opts)
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
	sys, err := contextFromImageOptions(c, opts)
	if err != nil {
		return nil, err
	}
	return ref.NewImageSource(ctx, sys)
}
