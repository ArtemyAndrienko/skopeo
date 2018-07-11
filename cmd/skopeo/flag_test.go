package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func TestOptionalBoolSet(t *testing.T) {
	for _, c := range []struct {
		input    string
		accepted bool
		value    bool
	}{
		// Valid inputs documented for strconv.ParseBool == flag.BoolVar
		{"1", true, true},
		{"t", true, true},
		{"T", true, true},
		{"TRUE", true, true},
		{"true", true, true},
		{"True", true, true},
		{"0", true, false},
		{"f", true, false},
		{"F", true, false},
		{"FALSE", true, false},
		{"false", true, false},
		{"False", true, false},
		// A few invalid inputs
		{"", false, false},
		{"yes", false, false},
		{"no", false, false},
		{"2", false, false},
	} {
		var ob optionalBool
		v := newOptionalBoolValue(&ob)
		require.False(t, ob.present)
		err := v.Set(c.input)
		if c.accepted {
			assert.NoError(t, err, c.input)
			assert.Equal(t, c.value, ob.value)
		} else {
			assert.Error(t, err, c.input)
			assert.False(t, ob.present) // Just to be extra paranoid.
		}
	}

	// Nothing actually explicitly says that .Set() is never called when the flag is not present on the command line;
	// so, check that it is not being called, at least in the straightforward case (it's not possible to test that it
	// is not called in any possible situation).
	var globalOB, commandOB optionalBool
	actionRun := false
	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{
		cli.GenericFlag{
			Name:  "global-OB",
			Value: newOptionalBoolValue(&globalOB),
		},
	}
	app.Commands = []cli.Command{{
		Name: "cmd",
		Flags: []cli.Flag{
			cli.GenericFlag{
				Name:  "command-OB",
				Value: newOptionalBoolValue(&commandOB),
			},
		},
		Action: func(*cli.Context) error {
			assert.False(t, globalOB.present)
			assert.False(t, commandOB.present)
			actionRun = true
			return nil
		},
	}}
	err := app.Run([]string{"app", "cmd"})
	require.NoError(t, err)
	assert.True(t, actionRun)
}

func TestOptionalBoolString(t *testing.T) {
	for _, c := range []struct {
		input    optionalBool
		expected string
	}{
		{optionalBool{present: true, value: true}, "true"},
		{optionalBool{present: true, value: false}, "false"},
		{optionalBool{present: false, value: true}, ""},
		{optionalBool{present: false, value: false}, ""},
	} {
		var ob optionalBool
		v := newOptionalBoolValue(&ob)
		ob = c.input
		res := v.String()
		assert.Equal(t, c.expected, res)
	}
}

func TestOptionalBoolIsBoolFlag(t *testing.T) {
	// IsBoolFlag means that the argument value must either be part of the same argument, with =;
	// if there is no =, the value is set to true.
	// This differs form other flags, where the argument is required and may be either separated with = or supplied in the next argument.
	for _, c := range []struct {
		input        []string
		expectedOB   optionalBool
		expectedArgs []string
	}{
		{[]string{"1", "2"}, optionalBool{present: false}, []string{"1", "2"}},                                       // Flag not present
		{[]string{"--OB=true", "1", "2"}, optionalBool{present: true, value: true}, []string{"1", "2"}},              // --OB=true
		{[]string{"--OB=false", "1", "2"}, optionalBool{present: true, value: false}, []string{"1", "2"}},            // --OB=false
		{[]string{"--OB", "true", "1", "2"}, optionalBool{present: true, value: true}, []string{"true", "1", "2"}},   // --OB true
		{[]string{"--OB", "false", "1", "2"}, optionalBool{present: true, value: true}, []string{"false", "1", "2"}}, // --OB false
	} {
		var ob optionalBool
		actionRun := false
		app := cli.NewApp()
		app.Commands = []cli.Command{{
			Name: "cmd",
			Flags: []cli.Flag{
				cli.GenericFlag{
					Name:  "OB",
					Value: newOptionalBoolValue(&ob),
				},
			},
			Action: func(ctx *cli.Context) error {
				assert.Equal(t, c.expectedOB, ob)
				assert.Equal(t, c.expectedArgs, ([]string)(ctx.Args()))
				actionRun = true
				return nil
			},
		}}
		err := app.Run(append([]string{"app", "cmd"}, c.input...))
		require.NoError(t, err)
		assert.True(t, actionRun)
	}
}
