package main

import (
	"context"
	"strings"
	"testing"
)

func TestPackBuilderRequiresExplicitTargetAndSafeVersion(t *testing.T) {
	for _, args := range [][]string{nil, {"--output", "relative", "--version", "development"}, {"--output", t.TempDir(), "--version", "unsafe -X another=value"}, {"--output", t.TempDir(), "--version", "development", "--goos", "unknown"}} {
		if err := run(context.Background(), args); err == nil {
			t.Fatal("unsafe pack build accepted", args)
		}
	}
}

func TestDarwinPackRequiresNativeACLBuild(t *testing.T) {
	for _, host := range []string{"windows", "linux"} {
		if _, err := executorBuildEnv(nil, host, "darwin", "arm64"); err == nil {
			t.Fatal("unsupported Darwin cross-build accepted", host)
		}
	}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		env, err := executorBuildEnv([]string{"CGO_ENABLED=0"}, platform, platform, "amd64")
		if err != nil {
			t.Fatal(err)
		}
		values := map[string]string{}
		for _, pair := range env {
			key, value, _ := strings.Cut(pair, "=")
			values[key] = value
		}
		want := "0"
		if platform == "darwin" {
			want = "1"
		}
		if values["CGO_ENABLED"] != want || values["GOOS"] != platform || values["GOARCH"] != "amd64" {
			t.Fatal(values)
		}
		if platform == "darwin" && values["CC"] != "clang" {
			t.Fatal("Darwin SDK compiler missing", values)
		}
	}
}
