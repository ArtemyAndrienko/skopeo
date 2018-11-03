package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containers/image/signature"
	"github.com/containers/skopeo/version"
	"github.com/containers/storage/pkg/reexec"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile
var gitCommit = ""

type globalOptions struct {
	debug          bool          // Enable debug output
	policyPath     string        // Path to a signature verification policy file
	insecurePolicy bool          // Use an "allow everything" signature verification policy
	commandTimeout time.Duration // Timeout for the command execution
}

// createApp returns a cli.App to be run or tested.
func createApp() *cli.App {
	opts := globalOptions{}

	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Name = "skopeo"
	if gitCommit != "" {
		app.Version = fmt.Sprintf("%s commit: %s", version.Version, gitCommit)
	} else {
		app.Version = version.Version
	}
	app.Usage = "Various operations with container images and container image registries"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "enable debug output",
			Destination: &opts.debug,
		},
		cli.BoolTFlag{
			Name:   "tls-verify",
			Usage:  "require HTTPS and verify certificates when talking to container registries (defaults to true)",
			Hidden: true,
		},
		cli.StringFlag{
			Name:        "policy",
			Usage:       "Path to a trust policy file",
			Destination: &opts.policyPath,
		},
		cli.BoolFlag{
			Name:        "insecure-policy",
			Usage:       "run the tool without any policy check",
			Destination: &opts.insecurePolicy,
		},
		cli.StringFlag{
			Name:  "registries.d",
			Value: "",
			Usage: "use registry configuration files in `DIR` (e.g. for container signature storage)",
		},
		cli.StringFlag{
			Name:  "override-arch",
			Value: "",
			Usage: "use `ARCH` instead of the architecture of the machine for choosing images",
		},
		cli.StringFlag{
			Name:  "override-os",
			Value: "",
			Usage: "use `OS` instead of the running OS for choosing images",
		},
		cli.DurationFlag{
			Name:        "command-timeout",
			Usage:       "timeout for the command execution",
			Destination: &opts.commandTimeout,
		},
	}
	app.Before = opts.before
	app.Commands = []cli.Command{
		copyCmd(&opts),
		inspectCmd(&opts),
		layersCmd(&opts),
		deleteCmd(&opts),
		manifestDigestCmd(),
		standaloneSignCmd(),
		standaloneVerifyCmd(),
		untrustedSignatureDumpCmd(),
	}
	return app
}

// before is run by the cli package for any command, before running the command-specific handler.
func (opts *globalOptions) before(c *cli.Context) error {
	if opts.debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if c.GlobalIsSet("tls-verify") {
		logrus.Warn("'--tls-verify' is deprecated, please set this on the specific subcommand")
	}
	return nil
}

func main() {
	if reexec.Init() {
		return
	}
	app := createApp()
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

// getPolicyContext returns a *signature.PolicyContext based on opts.
func (opts *globalOptions) getPolicyContext() (*signature.PolicyContext, error) {
	var policy *signature.Policy // This could be cached across calls in opts.
	var err error
	if opts.insecurePolicy {
		policy = &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	} else if opts.policyPath == "" {
		policy, err = signature.DefaultPolicy(nil)
	} else {
		policy, err = signature.NewPolicyFromFile(opts.policyPath)
	}
	if err != nil {
		return nil, err
	}
	return signature.NewPolicyContext(policy)
}

// commandTimeoutContext returns a context.Context and a cancellation callback based on opts.
// The caller should usually "defer cancel()" immediately after calling this.
func (opts *globalOptions) commandTimeoutContext() (context.Context, context.CancelFunc) {
	ctx := context.Background()
	var cancel context.CancelFunc = func() {}
	if opts.commandTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.commandTimeout)
	}
	return ctx, cancel
}
