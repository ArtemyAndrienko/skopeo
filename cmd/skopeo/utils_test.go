package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

// fakeGlobalOptions creates globalOptions and sets it according to flags.
// NOTE: This is QUITE FAKE; none of the urfave/cli normalization and the like happens.
func fakeGlobalOptions(t *testing.T, flags []string) (*cli.App, *cli.Context, *globalOptions) {
	app, opts := createApp()

	flagSet := flag.NewFlagSet(app.Name, flag.ContinueOnError)
	for _, f := range app.Flags {
		f.Apply(flagSet)
	}
	err := flagSet.Parse(flags)
	require.NoError(t, err)
	ctx := cli.NewContext(app, flagSet, nil)

	return app, ctx, opts
}

// fakeImageContext creates inputs for contextFromImageOptions.
// NOTE: This is QUITE FAKE; none of the urfave/cli normalization and the like happens.
func fakeImageContext(t *testing.T, cmdName string, flagPrefix string, globalFlags []string, cmdFlags []string) (*cli.Context, *imageOptions) {
	app, globalCtx, globalOpts := fakeGlobalOptions(t, globalFlags)

	cmd := app.Command(cmdName)
	require.NotNil(t, cmd)

	imageFlags, imageOpts := imageFlags(globalOpts, flagPrefix, "")
	appliedFlags := map[string]struct{}{}
	// Ugly: cmd.Flags includes imageFlags as well.  For now, we need cmd.Flags to apply here
	// to be able to test the non-Destination: flags, but we must not apply the same flag name twice.
	// So, primarily use imageFlags (so that Destination: is used as expected), and then follow up with
	// the remaining flags from cmd.Flags (so that cli.Context.String() etc. works).
	// This is horribly ugly, but all of this will disappear within this PR.
	firstName := func(f cli.Flag) string { // We even need to recognize "dest-creds,dcreds".  This will disappear as well.
		return strings.Split(f.GetName(), ",")[0]
	}
	flagSet := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
	for _, f := range imageFlags {
		f.Apply(flagSet)
		appliedFlags[firstName(f)] = struct{}{}
	}
	for _, f := range cmd.Flags {
		if _, ok := appliedFlags[firstName(f)]; !ok {
			f.Apply(flagSet)
			appliedFlags[firstName(f)] = struct{}{}
		}
	}

	err := flagSet.Parse(cmdFlags)
	require.NoError(t, err)
	return cli.NewContext(app, flagSet, globalCtx), imageOpts
}

func TestContextFromImageOptions(t *testing.T) {
	// FIXME: All of this only tests (skopeo copy --dest)

	// Default state
	c, opts := fakeImageContext(t, "copy", "dest-", []string{}, []string{})
	res, err := contextFromImageOptions(c, opts)
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{}, res)

	// Explicitly set everything to default, except for when the default is “not present”
	c, opts = fakeImageContext(t, "copy", "dest-", []string{}, []string{
		"--dest-compress=false",
	})
	res, err = contextFromImageOptions(c, opts)
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{}, res)

	// Set everything to non-default values.
	c, opts = fakeImageContext(t, "copy", "dest-", []string{
		"--registries.d", "/srv/registries.d",
		"--override-arch", "overridden-arch",
		"--override-os", "overridden-os",
	}, []string{
		"--authfile", "/srv/authfile",
		"--dest-cert-dir", "/srv/cert-dir",
		"--dest-shared-blob-dir", "/srv/shared-blob-dir",
		"--dest-compress=true",
		"--dest-daemon-host", "daemon-host.example.com",
		"--dest-tls-verify=false",
		"--dest-creds", "creds-user:creds-password",
	})
	res, err = contextFromImageOptions(c, opts)
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{
		RegistriesDirPath:                 "/srv/registries.d",
		AuthFilePath:                      "/srv/authfile",
		ArchitectureChoice:                "overridden-arch",
		OSChoice:                          "overridden-os",
		OCISharedBlobDirPath:              "/srv/shared-blob-dir",
		DockerCertPath:                    "/srv/cert-dir",
		DockerInsecureSkipTLSVerify:       types.OptionalBoolTrue,
		DockerAuthConfig:                  &types.DockerAuthConfig{Username: "creds-user", Password: "creds-password"},
		DockerDaemonCertPath:              "/srv/cert-dir",
		DockerDaemonHost:                  "daemon-host.example.com",
		DockerDaemonInsecureSkipTLSVerify: true,
		DirForceCompress:                  true,
	}, res)

	// Global/per-command tlsVerify behavior
	for _, c := range []struct {
		global, cmd          string
		expectedDocker       types.OptionalBool
		expectedDockerDaemon bool
	}{
		{"", "", types.OptionalBoolUndefined, false},
		{"", "false", types.OptionalBoolTrue, true},
		{"", "true", types.OptionalBoolFalse, false},
		{"false", "", types.OptionalBoolTrue, false},
		{"false", "false", types.OptionalBoolTrue, true},
		{"false", "true", types.OptionalBoolFalse, false},
		{"true", "", types.OptionalBoolFalse, false},
		{"true", "false", types.OptionalBoolTrue, true},
		{"true", "true", types.OptionalBoolFalse, false},
	} {
		globalFlags := []string{}
		if c.global != "" {
			globalFlags = append(globalFlags, "--tls-verify="+c.global)
		}
		cmdFlags := []string{}
		if c.cmd != "" {
			cmdFlags = append(cmdFlags, "--dest-tls-verify="+c.cmd)
		}
		ctx, opts := fakeImageContext(t, "copy", "dest-", globalFlags, cmdFlags)
		res, err = contextFromImageOptions(ctx, opts)
		require.NoError(t, err)
		assert.Equal(t, c.expectedDocker, res.DockerInsecureSkipTLSVerify, "%#v", c)
		assert.Equal(t, c.expectedDockerDaemon, res.DockerDaemonInsecureSkipTLSVerify, "%#v", c)
	}

	// Invalid option values
	c, opts = fakeImageContext(t, "copy", "dest-", []string{}, []string{"--dest-creds", ""})
	_, err = contextFromImageOptions(c, opts)
	assert.Error(t, err)
}

// fakeImageDestContext creates inputs for contextFromImageDestOptions.
// NOTE: This is QUITE FAKE; none of the urfave/cli normalization and the like happens.
func fakeImageDestContext(t *testing.T, cmdName string, flagPrefix string, globalFlags []string, cmdFlags []string) (*cli.Context, *imageDestOptions) {
	app, globalCtx, globalOpts := fakeGlobalOptions(t, globalFlags)

	cmd := app.Command(cmdName)
	require.NotNil(t, cmd)

	imageFlags, imageOpts := imageDestFlags(globalOpts, flagPrefix, "")
	appliedFlags := map[string]struct{}{}
	// Ugly: cmd.Flags includes imageFlags as well.  For now, we need cmd.Flags to apply here
	// to be able to test the non-Destination: flags, but we must not apply the same flag name twice.
	// So, primarily use imageFlags (so that Destination: is used as expected), and then follow up with
	// the remaining flags from cmd.Flags (so that cli.Context.String() etc. works).
	// This is horribly ugly, but all of this will disappear within this PR.
	firstName := func(f cli.Flag) string { // We even need to recognize "dest-creds,dcreds".  This will disappear as well.
		return strings.Split(f.GetName(), ",")[0]
	}
	flagSet := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
	for _, f := range imageFlags {
		f.Apply(flagSet)
		appliedFlags[firstName(f)] = struct{}{}
	}
	for _, f := range cmd.Flags {
		if _, ok := appliedFlags[firstName(f)]; !ok {
			f.Apply(flagSet)
			appliedFlags[firstName(f)] = struct{}{}
		}
	}

	err := flagSet.Parse(cmdFlags)
	require.NoError(t, err)
	return cli.NewContext(app, flagSet, globalCtx), imageOpts
}

func TestContextFromImageDestOptions(t *testing.T) {
	// FIXME: All of this only tests (skopeo copy --dest)

	// Default state
	c, opts := fakeImageDestContext(t, "copy", "dest-", []string{}, []string{})
	res, err := contextFromImageDestOptions(c, opts)
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{}, res)

	// Explicitly set everything to default, except for when the default is “not present”
	c, opts = fakeImageDestContext(t, "copy", "dest-", []string{}, []string{
		"--dest-compress=false",
	})
	res, err = contextFromImageDestOptions(c, opts)
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{}, res)

	// Set everything to non-default values.
	c, opts = fakeImageDestContext(t, "copy", "dest-", []string{
		"--registries.d", "/srv/registries.d",
		"--override-arch", "overridden-arch",
		"--override-os", "overridden-os",
	}, []string{
		"--authfile", "/srv/authfile",
		"--dest-cert-dir", "/srv/cert-dir",
		"--dest-ostree-tmp-dir", "/srv/ostree-tmp-dir",
		"--dest-shared-blob-dir", "/srv/shared-blob-dir",
		"--dest-compress=true",
		"--dest-daemon-host", "daemon-host.example.com",
		"--dest-tls-verify=false",
		"--dest-creds", "creds-user:creds-password",
	})
	res, err = contextFromImageDestOptions(c, opts)
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{
		RegistriesDirPath:                 "/srv/registries.d",
		AuthFilePath:                      "/srv/authfile",
		ArchitectureChoice:                "overridden-arch",
		OSChoice:                          "overridden-os",
		OCISharedBlobDirPath:              "/srv/shared-blob-dir",
		DockerCertPath:                    "/srv/cert-dir",
		DockerInsecureSkipTLSVerify:       types.OptionalBoolTrue,
		DockerAuthConfig:                  &types.DockerAuthConfig{Username: "creds-user", Password: "creds-password"},
		OSTreeTmpDirPath:                  "/srv/ostree-tmp-dir",
		DockerDaemonCertPath:              "/srv/cert-dir",
		DockerDaemonHost:                  "daemon-host.example.com",
		DockerDaemonInsecureSkipTLSVerify: true,
		DirForceCompress:                  true,
	}, res)

	// Invalid option values in imageOptions
	c, opts = fakeImageDestContext(t, "copy", "dest-", []string{}, []string{"--dest-creds", ""})
	_, err = contextFromImageDestOptions(c, opts)
	assert.Error(t, err)
}
