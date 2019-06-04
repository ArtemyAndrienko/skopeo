#!/usr/bin/env bats
#
# Simplest test for skopeo inspect
#

load helpers

@test "inspect: basic" {
    workdir=$TESTDIR/inspect

    remote_image=docker://quay.io/libpod/alpine_labels:latest
    # Inspect remote source, then pull it. There's a small race condition
    # in which the remote image can get updated between the inspect and
    # the copy; let's just not worry about it.
    run_skopeo inspect $remote_image
    inspect_remote=$output

    # Now pull it into a directory
    run_skopeo copy $remote_image dir:$workdir
    expect_output --substring "Getting image source signatures"
    expect_output --substring "Writing manifest to image destination"

    # Unpacked contents must include a manifest and version
    [ -e $workdir/manifest.json ]
    [ -e $workdir/version ]

    # Now run inspect locally
    run_skopeo inspect dir:$workdir
    inspect_local=$output

    # Each SHA-named file must be listed in the output of 'inspect'
    for sha in $(find $workdir -type f | xargs -l1 basename | egrep '^[0-9a-f]{64}$'); do
        expect_output --from="$inspect_local" --substring "sha256:$sha" \
                      "Locally-extracted SHA file is present in 'inspect'"
    done

    # Simple sanity check on 'inspect' output.
    # For each of the given keys (LHS of the table below):
    #    1) Get local and remote values
    #    2) Sanity-check local value using simple expression
    #    3) Confirm that local and remote values match.
    #
    # The reason for (2) is to make sure that we don't compare bad results
    #
    # The reason for a hardcoded list, instead of 'jq keys', is that RepoTags
    # is always empty locally, but a list remotely.
    while read key expect; do
        local=$(echo "$inspect_local" | jq -r ".$key")
        remote=$(echo "$inspect_remote" | jq -r ".$key")

        expect_output --from="$local" --substring "$expect" \
                  "local $key is sane"

        expect_output --from="$remote" "$local" \
                      "local $key matches remote"
    done <<END_EXPECT
Architecture       amd64
Created            [0-9-]+T[0-9:]+\.[0-9]+Z
Digest             sha256:[0-9a-f]{64}
DockerVersion      [0-9]+\.[0-9][0-9.-]+
Labels             \\\{.*PODMAN.*podman.*\\\}
Layers             \\\[.*sha256:.*\\\]
Os                 linux
END_EXPECT
}

# vim: filetype=sh
