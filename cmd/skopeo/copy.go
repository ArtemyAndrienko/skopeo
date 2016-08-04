package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"strings"

	"github.com/containers/image/image"
	"github.com/containers/image/signature"
	"github.com/containers/image/transports"
	"github.com/urfave/cli"
)

// supportedDigests lists the supported blob digest types.
var supportedDigests = map[string]func() hash.Hash{
	"sha256": sha256.New,
}

type digestingReader struct {
	source           io.Reader
	digest           hash.Hash
	expectedDigest   []byte
	failureIndicator *bool
}

// newDigestingReader returns an io.Reader with contents of source, which will eventually return a non-EOF error
// and set *failureIndicator to true if the source stream does not match expectedDigestString.
func newDigestingReader(source io.Reader, expectedDigestString string, failureIndicator *bool) (io.Reader, error) {
	fields := strings.SplitN(expectedDigestString, ":", 2)
	if len(fields) != 2 {
		return nil, fmt.Errorf("Invalid digest specification %s", expectedDigestString)
	}
	fn, ok := supportedDigests[fields[0]]
	if !ok {
		return nil, fmt.Errorf("Invalid digest specification %s: unknown digest type %s", expectedDigestString, fields[0])
	}
	digest := fn()
	expectedDigest, err := hex.DecodeString(fields[1])
	if err != nil {
		return nil, fmt.Errorf("Invalid digest value %s: %v", expectedDigestString, err)
	}
	if len(expectedDigest) != digest.Size() {
		return nil, fmt.Errorf("Invalid digest specification %s: length %d does not match %d", expectedDigestString, len(expectedDigest), digest.Size())
	}
	return &digestingReader{
		source:           source,
		digest:           digest,
		expectedDigest:   expectedDigest,
		failureIndicator: failureIndicator,
	}, nil
}

func (d *digestingReader) Read(p []byte) (int, error) {
	n, err := d.source.Read(p)
	if n > 0 {
		if n2, err := d.digest.Write(p[:n]); n2 != n || err != nil {
			// Coverage: This should not happen, the hash.Hash interface requires
			// d.digest.Write to never return an error, and the io.Writer interface
			// requires n2 == len(input) if no error is returned.
			return 0, fmt.Errorf("Error updating digest during verification: %d vs. %d, %v", n2, n, err)
		}
	}
	if err == io.EOF {
		actualDigest := d.digest.Sum(nil)
		if subtle.ConstantTimeCompare(actualDigest, d.expectedDigest) != 1 {
			*d.failureIndicator = true
			return 0, fmt.Errorf("Digest did not match, expected %s, got %s", hex.EncodeToString(d.expectedDigest), hex.EncodeToString(actualDigest))
		}
	}
	return n, err
}

func copyHandler(context *cli.Context) error {
	if len(context.Args()) != 2 {
		return errors.New("Usage: copy source destination")
	}

	dest, err := parseImageDestination(context, context.Args()[1])
	if err != nil {
		return fmt.Errorf("Error initializing %s: %v", context.Args()[1], err)
	}

	rawSource, err := parseImageSource(context, context.Args()[0])
	if err != nil {
		return fmt.Errorf("Error initializing %s: %v", context.Args()[0], err)
	}
	src := image.FromSource(rawSource, dest.SupportedManifestMIMETypes())

	signBy := context.String("sign-by")

	manifest, _, err := src.Manifest()
	if err != nil {
		return fmt.Errorf("Error reading manifest: %v", err)
	}

	blobDigests, err := src.BlobDigests()
	if err != nil {
		return fmt.Errorf("Error parsing manifest: %v", err)
	}
	for _, digest := range blobDigests {
		// TODO(mitr): do not ignore the size param returned here
		stream, _, err := rawSource.GetBlob(digest)
		if err != nil {
			return fmt.Errorf("Error reading blob %s: %v", digest, err)
		}
		defer stream.Close()

		// Be paranoid; in case PutBlob somehow managed to ignore an error from digestingReader,
		// use a separate validation failure indicator.
		// Note that we don't use a stronger "validationSucceeded" indicator, because
		// dest.PutBlob may detect that the layer already exists, in which case we don't
		// read stream to the end, and validation does not happen.
		validationFailed := false // This is a new instance on each loop iteration.
		digestingReader, err := newDigestingReader(stream, digest, &validationFailed)
		if err != nil {
			return fmt.Errorf("Error preparing to verify blob %s: %v", digest, err)
		}
		if err := dest.PutBlob(digest, digestingReader); err != nil {
			return fmt.Errorf("Error writing blob: %v", err)
		}
		if validationFailed { // Coverage: This should never happen.
			return fmt.Errorf("Internal error uploading blob %s, digest verification failed but was ignored", digest)
		}
	}

	sigs, err := src.Signatures()
	if err != nil {
		return fmt.Errorf("Error reading signatures: %v", err)
	}

	if signBy != "" {
		mech, err := signature.NewGPGSigningMechanism()
		if err != nil {
			return fmt.Errorf("Error initializing GPG: %v", err)
		}
		dockerReference := dest.Reference().DockerReference()
		if dockerReference == nil {
			return fmt.Errorf("Cannot determine canonical Docker reference for destination %s", transports.ImageName(dest.Reference()))
		}

		newSig, err := signature.SignDockerManifest(manifest, dockerReference.String(), mech, signBy)
		if err != nil {
			return fmt.Errorf("Error creating signature: %v", err)
		}
		sigs = append(sigs, newSig)
	}

	// FIXME: We need to call PutManifest after PutBlob and before PutSignatures. This seems ugly; move to a "set properties" + "commit" model?
	if err := dest.PutManifest(manifest); err != nil {
		return fmt.Errorf("Error writing manifest: %v", err)
	}

	if err := dest.PutSignatures(sigs); err != nil {
		return fmt.Errorf("Error writing signatures: %v", err)
	}
	return nil
}

var copyCmd = cli.Command{
	Name:   "copy",
	Action: copyHandler,
	// FIXME: Do we need to namespace the GPG aspect?
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "sign-by",
			Usage: "sign the image using a GPG key with the specified fingerprint",
		},
	},
}
