// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package starlarkutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetExistingMirrorsFromDepsBzl(t *testing.T) {
	depsbzl := `# leading comment
load("@bazel_gazelle//:deps.bzl", "go_repository")
def go_deps():
    go_repository(
        name = "com_github_akavel_rsrc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/akavel/rsrc",
        sum = "h1:zjWn7ukO9Kc5Q62DOJCcxGpXC18RawVtYAGdz2aLlfw=",
        version = "v0.8.0",
    )
    go_repository(
        name = "com_github_alecthomas_units",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/units",
        sha256 = "abcdefghij",
        sum = "h1:AUNCr9CiJuwrRYS3XieqF+Z9B9gNxo/eANAJCF2eiN4=",
        urls = ["https://foo/bar.zip"],
    )
`
	mirrors, err := downloadableArtifactsFromDepsBzl(depsbzl)
	require.NoError(t, err)
	require.Equal(t, len(mirrors), 1)
	mirror := mirrors["com_github_alecthomas_units"]
	require.Equal(t, mirror.URL, "https://foo/bar.zip")
	require.Equal(t, mirror.Sha256, "abcdefghij")
}
