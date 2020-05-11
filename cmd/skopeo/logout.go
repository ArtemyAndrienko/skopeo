package main

import (
	"io"

	"github.com/containers/common/pkg/auth"
	"github.com/spf13/cobra"
)

type logoutOptions struct {
	global     *globalOptions
	logoutOpts auth.LogoutOptions
}

func logoutCmd(global *globalOptions) *cobra.Command {
	opts := logoutOptions{
		global: global,
	}
	cmd := &cobra.Command{
		Use:     "logout",
		Short:   "Logout of a container registry",
		Long:    "Logout of a container registry on a specified server.",
		RunE:    commandAction(opts.run),
		Example: `skopeo logout quay.io`,
	}
	adjustUsage(cmd)
	cmd.Flags().AddFlagSet(auth.GetLogoutFlags(&opts.logoutOpts))
	return cmd
}

func (opts *logoutOptions) run(args []string, stdout io.Writer) error {
	opts.logoutOpts.Stdout = stdout
	sys := opts.global.newSystemContext()
	return auth.Logout(sys, &opts.logoutOpts, args)
}
