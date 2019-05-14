#!/usr/bin/env bats
#
# Tests with a local registry with auth
#

load helpers

function setup() {
    standard_setup

    # Remove old/stale cred file
    _cred_file=${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/containers/auth.json
    rm -f $_cred_file

    # Start authenticated registry with random password
    testuser=testuser
    testpassword=$(random_string 15)

    start_registry --testuser=testuser --testpassword=$testpassword reg
}

@test "auth: credentials on command line" {
    # No creds
    run_skopeo 1 inspect --tls-verify=false docker://localhost:5000/nonesuch
    expect_output --substring "unauthorized: authentication required"

    # Wrong user
    run_skopeo 1 inspect --tls-verify=false --creds=baduser:badpassword \
               docker://localhost:5000/nonesuch
    expect_output --substring "unauthorized: authentication required"

    # Wrong password
    run_skopeo 1 inspect --tls-verify=false --creds=$testuser:badpassword \
               docker://localhost:5000/nonesuch
    expect_output --substring "unauthorized: authentication required"

    # Correct creds, but no such image
    run_skopeo 1 inspect --tls-verify=false --creds=$testuser:$testpassword \
               docker://localhost:5000/nonesuch
    expect_output --substring "manifest unknown: manifest unknown"

    # These should pass
    run_skopeo copy --dest-tls-verify=false --dcreds=$testuser:$testpassword \
               docker://busybox:latest docker://localhost:5000/busybox:mine
    run_skopeo inspect --tls-verify=false --creds=$testuser:$testpassword \
               docker://localhost:5000/busybox:mine
    expect_output --substring "localhost:5000/busybox"
}

@test "auth: credentials via podman login" {
    # Logged in: skopeo should work
    podman login --tls-verify=false -u $testuser -p $testpassword localhost:5000

    run_skopeo copy --dest-tls-verify=false \
               docker://busybox:latest docker://localhost:5000/busybox:mine
    run_skopeo inspect --tls-verify=false docker://localhost:5000/busybox:mine
    expect_output --substring "localhost:5000/busybox"

    # Logged out: should fail
    podman logout localhost:5000

    run_skopeo 1 inspect --tls-verify=false docker://localhost:5000/busybox:mine
    expect_output --substring "unauthorized: authentication required"
}

teardown() {
    podman rm -f reg

    if [[ -n $_cred_file ]]; then
        rm -f $_cred_file
    fi

    standard_teardown
}

# vim: filetype=sh
