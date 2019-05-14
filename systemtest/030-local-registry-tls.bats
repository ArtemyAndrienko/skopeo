#!/usr/bin/env bats
#
# This is probably a never-mind.
#
# The idea is to set up a local registry with locally generated certs,
# using --dest-cert-dir to tell skopeo how to check. But no, it fails with
#
#        x509: certificate signed by unknown authority
#
# Perhaps I'm missing something? Maybe I need to add something into
# /etc/pki/somewhere? If this is truly not possible to test without
# a real signature, then let's just delete this test.
#
#
load helpers

function setup() {
    standard_setup

    start_registry --with-cert reg
}

@test "local registry, with cert" {
    skip "doesn't work as expected"

    local remote_image=docker://busybox:latest
    local localimg=docker://localhost:5000/busybox:unsigned

    # Fails with: x509: certificate signed by unknown authority
    run_skopeo --debug copy --dest-cert-dir=$TESTDIR/auth \
               docker://busybox:latest \
               docker://localhost:5000/busybox:unsigned
}

teardown() {
    podman rm -f reg

    standard_teardown
}

# vim: filetype=sh
