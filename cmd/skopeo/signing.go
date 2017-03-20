package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/containers/image/signature"
	"github.com/urfave/cli"
)

func standaloneSign(context *cli.Context) error {
	outputFile := context.String("output")
	if len(context.Args()) != 3 || outputFile == "" {
		return errors.New("Usage: skopeo standalone-sign manifest docker-reference key-fingerprint -o signature")
	}
	manifestPath := context.Args()[0]
	dockerReference := context.Args()[1]
	fingerprint := context.Args()[2]

	manifest, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("Error reading %s: %v", manifestPath, err)
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return fmt.Errorf("Error initializing GPG: %v", err)
	}
	defer mech.Close()
	signature, err := signature.SignDockerManifest(manifest, dockerReference, mech, fingerprint)
	if err != nil {
		return fmt.Errorf("Error creating signature: %v", err)
	}

	if err := ioutil.WriteFile(outputFile, signature, 0644); err != nil {
		return fmt.Errorf("Error writing signature to %s: %v", outputFile, err)
	}
	return nil
}

var standaloneSignCmd = cli.Command{
	Name:      "standalone-sign",
	Usage:     "Create a signature using local files",
	ArgsUsage: "MANIFEST DOCKER-REFERENCE KEY-FINGERPRINT",
	Action:    standaloneSign,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "output the signature to `SIGNATURE`",
		},
	},
}

func standaloneVerify(context *cli.Context) error {
	if len(context.Args()) != 4 {
		return errors.New("Usage: skopeo standalone-verify manifest docker-reference key-fingerprint signature")
	}
	manifestPath := context.Args()[0]
	expectedDockerReference := context.Args()[1]
	expectedFingerprint := context.Args()[2]
	signaturePath := context.Args()[3]

	unverifiedManifest, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("Error reading manifest from %s: %v", manifestPath, err)
	}
	unverifiedSignature, err := ioutil.ReadFile(signaturePath)
	if err != nil {
		return fmt.Errorf("Error reading signature from %s: %v", signaturePath, err)
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return fmt.Errorf("Error initializing GPG: %v", err)
	}
	defer mech.Close()
	sig, err := signature.VerifyDockerManifestSignature(unverifiedSignature, unverifiedManifest, expectedDockerReference, mech, expectedFingerprint)
	if err != nil {
		return fmt.Errorf("Error verifying signature: %v", err)
	}

	fmt.Fprintf(context.App.Writer, "Signature verified, digest %s\n", sig.DockerManifestDigest)
	return nil
}

var standaloneVerifyCmd = cli.Command{
	Name:      "standalone-verify",
	Usage:     "Verify a signature using local files",
	ArgsUsage: "MANIFEST DOCKER-REFERENCE KEY-FINGERPRINT SIGNATURE",
	Action:    standaloneVerify,
}

func untrustedSignatureDump(context *cli.Context) error {
	if len(context.Args()) != 1 {
		return errors.New("Usage: skopeo untrusted-signature-dump-without-verification signature")
	}
	untrustedSignaturePath := context.Args()[0]

	untrustedSignature, err := ioutil.ReadFile(untrustedSignaturePath)
	if err != nil {
		return fmt.Errorf("Error reading untrusted signature from %s: %v", untrustedSignaturePath, err)
	}

	untrustedInfo, err := signature.GetUntrustedSignatureInformationWithoutVerifying(untrustedSignature)
	if err != nil {
		return fmt.Errorf("Error decoding untrusted signature: %v", err)
	}
	untrustedOut, err := json.MarshalIndent(untrustedInfo, "", "    ")
	if err != nil {
		return err
	}
	fmt.Fprintln(context.App.Writer, string(untrustedOut))
	return nil
}

// WARNING: Do not use the contents of this for ANY security decisions,
// and be VERY CAREFUL about showing this information to humans in any way which suggest that these values “are probably” reliable.
// There is NO REASON to expect the values to be correct, or not intentionally misleading
// (including things like “✅ Verified by $authority”)
//
// The subcommand is undocumented, and it may be renamed or entirely disappear in the future.
var untrustedSignatureDumpCmd = cli.Command{
	Name:      "untrusted-signature-dump-without-verification",
	Usage:     "Dump contents of a signature WITHOUT VERIFYING IT",
	ArgsUsage: "SIGNATURE",
	Hidden:    true,
	Action:    untrustedSignatureDump,
}
