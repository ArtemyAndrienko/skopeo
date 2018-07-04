package main

import (
	"flag"
	"testing"

	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

// fakeContext creates inputs for contextFromGlobalOptions.
// NOTE: This is QUITE FAKE; none of the urfave/cli normalization and the like happens.
func fakeContext(t *testing.T, cmdName string, globalFlags []string, cmdFlags []string) *cli.Context {
	app := createApp()

	globalSet := flag.NewFlagSet(app.Name, flag.ContinueOnError)
	for _, f := range app.Flags {
		f.Apply(globalSet)
	}
	err := globalSet.Parse(globalFlags)
	require.NoError(t, err)
	globalCtx := cli.NewContext(app, globalSet, nil)

	cmd := app.Command(cmdName)
	require.NotNil(t, cmd)
	cmdSet := flag.NewFlagSet(cmd.Name, flag.ContinueOnError)
	for _, f := range cmd.Flags {
		f.Apply(cmdSet)
	}
	err = cmdSet.Parse(cmdFlags)
	require.NoError(t, err)
	return cli.NewContext(app, cmdSet, globalCtx)
}

func TestContextFromGlobalOptions(t *testing.T) {
	// FIXME: All of this only tests (skopeo copy --dest)
	// FIXME FIXME: Apparently BoolT values are set to false if the flag is not declared for the specific subcommand!!

	// Default state
	c := fakeContext(t, "copy", []string{}, []string{})
	res, err := contextFromGlobalOptions(c, "dest-")
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{}, res)

	// Explicitly set everything to default, except for when the default is “not present”
	c = fakeContext(t, "copy", []string{}, []string{
		"--dest-compress=false",
	})
	res, err = contextFromGlobalOptions(c, "dest-")
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{}, res)

	// Set everything to non-default values.
	c = fakeContext(t, "copy", []string{
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
	res, err = contextFromGlobalOptions(c, "dest-")
	require.NoError(t, err)
	assert.Equal(t, &types.SystemContext{
		RegistriesDirPath:                 "/srv/registries.d",
		AuthFilePath:                      "/srv/authfile",
		ArchitectureChoice:                "overridden-arch",
		OSChoice:                          "overridden-os",
		OCISharedBlobDirPath:              "/srv/shared-blob-dir",
		DockerCertPath:                    "/srv/cert-dir",
		DockerInsecureSkipTLSVerify:       true,
		DockerAuthConfig:                  &types.DockerAuthConfig{Username: "creds-user", Password: "creds-password"},
		OSTreeTmpDirPath:                  "/srv/ostree-tmp-dir",
		DockerDaemonCertPath:              "/srv/cert-dir",
		DockerDaemonHost:                  "daemon-host.example.com",
		DockerDaemonInsecureSkipTLSVerify: true,
		DirForceCompress:                  true,
	}, res)

	// Global/per-command tlsVerify behavior
	for _, c := range []struct {
		global, cmd                          string
		expectedDocker, expectedDockerDaemon bool
	}{
		{"", "", false, false},
		{"", "false", true, true},
		{"", "true", false, false},
		{"false", "", true, false},
		{"false", "false", true, true},
		{"false", "true", false, false},
		{"true", "", false, false},
		{"true", "false", true, true},
		{"true", "true", false, false},
	} {
		globalFlags := []string{}
		if c.global != "" {
			globalFlags = append(globalFlags, "--tls-verify="+c.global)
		}
		cmdFlags := []string{}
		if c.cmd != "" {
			cmdFlags = append(cmdFlags, "--dest-tls-verify="+c.cmd)
		}
		ctx := fakeContext(t, "copy", globalFlags, cmdFlags)
		res, err = contextFromGlobalOptions(ctx, "dest-")
		require.NoError(t, err)
		assert.Equal(t, c.expectedDocker, res.DockerInsecureSkipTLSVerify, "%#v", c)
		assert.Equal(t, c.expectedDockerDaemon, res.DockerDaemonInsecureSkipTLSVerify, "%#v", c)
	}

	// Invalid option values
	c = fakeContext(t, "copy", []string{}, []string{"--dest-creds", ""})
	_, err = contextFromGlobalOptions(c, "dest-")
	assert.Error(t, err)
}
