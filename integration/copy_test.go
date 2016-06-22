package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/manifest"
	"github.com/go-check/check"
)

func init() {
	check.Suite(&CopySuite{})
}

type CopySuite struct {
	cluster *openshiftCluster
	gpgHome string
}

func (s *CopySuite) SetUpSuite(c *check.C) {
	if os.Getenv("SKOPEO_CONTAINER_TESTS") != "1" {
		c.Skip("Not running in a container, refusing to affect user state")
	}

	s.cluster = startOpenshiftCluster(c)

	for _, stream := range []string{"unsigned", "personal", "official", "naming", "cosigned"} {
		isJSON := fmt.Sprintf(`{
			"kind": "ImageStream",
			"apiVersion": "v1",
			"metadata": {
			    "name": "%s"
			},
			"spec": {}
		}`, stream)
		runCommandWithInput(c, isJSON, "oc", "create", "-f", "-")
	}

	gpgHome, err := ioutil.TempDir("", "skopeo-gpg")
	c.Assert(err, check.IsNil)
	s.gpgHome = gpgHome
	os.Setenv("GNUPGHOME", s.gpgHome)

	for _, key := range []string{"personal", "official"} {
		batchInput := fmt.Sprintf("Key-Type: RSA\nName-Real: Test key - %s\nName-email: %s@example.com\n%%commit\n",
			key, key)
		runCommandWithInput(c, batchInput, gpgBinary, "--batch", "--gen-key")

		out := combinedOutputOfCommand(c, gpgBinary, "--armor", "--export", fmt.Sprintf("%s@example.com", key))
		err := ioutil.WriteFile(filepath.Join(s.gpgHome, fmt.Sprintf("%s-pubkey.gpg", key)),
			[]byte(out), 0600)
		c.Assert(err, check.IsNil)
	}
}

func (s *CopySuite) TearDownSuite(c *check.C) {
	if s.gpgHome != "" {
		os.RemoveAll(s.gpgHome)
	}
	if s.cluster != nil {
		s.cluster.tearDown()
	}
}

// preparePolicyFixture applies edits to fixtures/policy.json and returns a path to the temporary file.
// Callers should defer os.Remove(the_returned_path)
func preparePolicyFixture(c *check.C, edits map[string]string) string {
	commands := []string{}
	for template, value := range edits {
		commands = append(commands, fmt.Sprintf("s,%s,%s,g", template, value))
	}
	json := combinedOutputOfCommand(c, "sed", strings.Join(commands, "; "), "fixtures/policy.json")

	file, err := ioutil.TempFile("", "policy.json")
	c.Assert(err, check.IsNil)
	path := file.Name()

	_, err = file.Write([]byte(json))
	c.Assert(err, check.IsNil)
	err = file.Close()
	c.Assert(err, check.IsNil)
	return path
}

// The most basic (skopeo copy) use:
func (s *CopySuite) TestCopySimple(c *check.C) {
	dir1, err := ioutil.TempDir("", "copy-1")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir1)
	dir2, err := ioutil.TempDir("", "copy-2")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir2)

	// FIXME: It would be nice to use one of the local Docker registries instead of neeeding an Internet connection.
	// "pull": docker: → dir:
	assertSkopeoSucceeds(c, "", "copy", "docker://estesp/busybox:latest", "dir:"+dir1)
	// "push": dir: → atomic:
	assertSkopeoSucceeds(c, "", "--debug", "copy", "dir:"+dir1, "atomic:myns/unsigned:unsigned")
	// The result of pushing and pulling is an unmodified image.
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/unsigned:unsigned", "dir:"+dir2)
	out := combinedOutputOfCommand(c, "diff", "-urN", dir1, dir2)
	c.Assert(out, check.Equals, "")

	// docker v2s2 -> OCI image layout
	// ociDest will be created by oci: if it doesn't exist
	// so don't create it here to exercise auto-creation
	ociDest := "busybox-latest"
	defer os.RemoveAll(ociDest)
	assertSkopeoSucceeds(c, "", "copy", "docker://busybox:latest", "oci:"+ociDest)
	_, err = os.Stat(ociDest)
	c.Assert(err, check.IsNil)

	// FIXME: Also check pushing to docker://
}

// Streaming (skopeo copy)
func (s *CopySuite) TestCopyStreaming(c *check.C) {
	dir1, err := ioutil.TempDir("", "streaming-1")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir1)
	dir2, err := ioutil.TempDir("", "streaming-2")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir2)

	// FIXME: It would be nice to use one of the local Docker registries instead of neeeding an Internet connection.
	// streaming: docker: → atomic:
	assertSkopeoSucceeds(c, "", "--debug", "copy", "docker://estesp/busybox:amd64", "atomic:myns/unsigned:streaming")
	// Compare (copies of) the original and the copy:
	assertSkopeoSucceeds(c, "", "copy", "docker://estesp/busybox:amd64", "dir:"+dir1)
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/unsigned:streaming", "dir:"+dir2)
	// The manifests will have different JWS signatures; so, compare the manifests by digests, which
	// strips the signatures, and remove them, comparing the rest file by file.
	digests := []string{}
	for _, dir := range []string{dir1, dir2} {
		manifestPath := filepath.Join(dir, "manifest.json")
		m, err := ioutil.ReadFile(manifestPath)
		c.Assert(err, check.IsNil)
		digest, err := manifest.Digest(m)
		c.Assert(err, check.IsNil)
		digests = append(digests, digest)
		err = os.Remove(manifestPath)
		c.Assert(err, check.IsNil)
		c.Logf("Manifest file %s (digest %s) removed", manifestPath, digest)
	}
	c.Assert(digests[0], check.Equals, digests[1])
	out := combinedOutputOfCommand(c, "diff", "-urN", dir1, dir2)
	c.Assert(out, check.Equals, "")
	// FIXME: Also check pushing to docker://
}

// --sign-by and --policy copy, primarily using atomic:
func (s *CopySuite) TestCopySignatures(c *check.C) {
	dir, err := ioutil.TempDir("", "signatures-dest")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir)
	dirDest := "dir:" + dir

	policy := preparePolicyFixture(c, map[string]string{"@keydir@": s.gpgHome})
	defer os.Remove(policy)

	// type: reject
	assertSkopeoFails(c, ".*Source image rejected: Running image docker://busybox:latest is rejected by policy.*",
		"--policy", policy, "copy", "docker://busybox:latest", dirDest)

	// type: insecureAcceptAnything
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "docker://openshift/hello-openshift", dirDest)

	// type: signedBy
	// Sign the images
	assertSkopeoSucceeds(c, "", "copy", "--sign-by", "personal@example.com", "docker://busybox:1.23", "atomic:myns/personal:personal")
	assertSkopeoSucceeds(c, "", "copy", "--sign-by", "official@example.com", "docker://busybox:1.23.2", "atomic:myns/official:official")
	// Verify that we can pull them
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "atomic:myns/personal:personal", dirDest)
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "atomic:myns/official:official", dirDest)
	// Verify that mis-signed images are rejected
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/personal:personal", "atomic:myns/official:attack")
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/official:official", "atomic:myns/personal:attack")
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--policy", policy, "copy", "atomic:myns/personal:attack", dirDest)
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--policy", policy, "copy", "atomic:myns/official:attack", dirDest)

	// Verify that signed identity is verified.
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/official:official", "atomic:myns/naming:test1")
	assertSkopeoFails(c, ".*Source image rejected: Signature for identity localhost:8443/myns/official:official is not accepted.*",
		"--policy", policy, "copy", "atomic:myns/naming:test1", dirDest)
	// signedIdentity works
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/official:official", "atomic:myns/naming:naming")
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "atomic:myns/naming:naming", dirDest)

	// Verify that cosigning requirements are enforced
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/official:official", "atomic:myns/cosigned:cosigned")
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--policy", policy, "copy", "atomic:myns/cosigned:cosigned", dirDest)

	assertSkopeoSucceeds(c, "", "copy", "--sign-by", "personal@example.com", "atomic:myns/official:official", "atomic:myns/cosigned:cosigned")
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "atomic:myns/cosigned:cosigned", dirDest)
}

// --policy copy for dir: sources
func (s *CopySuite) TestCopyDirSignatures(c *check.C) {
	topDir, err := ioutil.TempDir("", "dir-signatures-top")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(topDir)
	topDirDest := "dir:" + topDir

	for _, suffix := range []string{"/dir1", "/dir2", "/restricted/personal", "/restricted/official", "/restricted/badidentity", "/dest"} {
		err := os.MkdirAll(topDir+suffix, 0755)
		c.Assert(err, check.IsNil)
	}

	// Note the "/@dirpath@": The value starts with a slash so that it is not rejected in other tests which do not replace it,
	// but we must ensure that the result is a canonical path, not something starting with a "//".
	policy := preparePolicyFixture(c, map[string]string{"@keydir@": s.gpgHome, "/@dirpath@": topDir + "/restricted"})
	defer os.Remove(policy)

	// Get some images.
	assertSkopeoSucceeds(c, "", "copy", "docker://estesp/busybox:armfh", topDirDest+"/dir1")
	assertSkopeoSucceeds(c, "", "copy", "docker://estesp/busybox:s390x", topDirDest+"/dir2")

	// Sign the images. By coping fom a topDirDest/dirN, also test that non-/restricted paths
	// use the dir:"" default of insecureAcceptAnything.
	// (For signing, we must push to atomic: to get a Docker identity to use in the signature.)
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "--sign-by", "personal@example.com", topDirDest+"/dir1", "atomic:myns/personal:dirstaging")
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "--sign-by", "official@example.com", topDirDest+"/dir2", "atomic:myns/official:dirstaging")
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/personal:dirstaging", topDirDest+"/restricted/personal")
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/official:dirstaging", topDirDest+"/restricted/official")

	// type: signedBy, with a signedIdentity override (necessary because dir: identities can't be signed)
	// Verify that correct images are accepted
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", topDirDest+"/restricted/official", topDirDest+"/dest")
	// ... and that mis-signed images are rejected.
	assertSkopeoFails(c, ".*Source image rejected: Invalid GPG signature.*",
		"--policy", policy, "copy", topDirDest+"/restricted/personal", topDirDest+"/dest")

	// Verify that the signed identity is verified.
	assertSkopeoSucceeds(c, "", "--policy", policy, "copy", "--sign-by", "official@example.com", topDirDest+"/dir1", "atomic:myns/personal:dirstaging2")
	assertSkopeoSucceeds(c, "", "copy", "atomic:myns/personal:dirstaging2", topDirDest+"/restricted/badidentity")
	assertSkopeoFails(c, ".*Source image rejected: .*Signature for identity localhost:8443/myns/personal:dirstaging2 is not accepted.*",
		"--policy", policy, "copy", topDirDest+"/restricted/badidentity", topDirDest+"/dest")
}
