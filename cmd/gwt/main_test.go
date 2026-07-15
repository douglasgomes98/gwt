package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"runtime/debug"
)

func TestVersionFromBuildInfo(t *testing.T) {
	for _, tc := range []struct {
		name, current, module, want string
	}{
		{"module version", "dev", "v0.1.0", "v0.1.0"},
		{"local build", "dev", "(devel)", "dev"},
		{"ldflags version", "v0.1.0", "v0.1.0", "v0.1.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionFromBuildInfo(tc.current, &debug.BuildInfo{Main: debug.Module{Version: tc.module}}); got != tc.want {
				t.Fatalf("versionFromBuildInfo() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMain(t *testing.T) {
	if os.Getenv("GWT_MAIN_HELPER") == "1" {
		if err := os.Chdir(os.Getenv("GWT_MAIN_DIR")); err != nil {
			t.Fatal(err)
		}
		os.Args = []string{"gwt", os.Getenv("GWT_MAIN_ARG")}
		main()
		return
	}
	for _, tc := range []struct {
		name string
		arg  string
		file string
		want string
	}{
		{name: "version", arg: "version", want: "dev"},
		{name: "config error", arg: "version", file: "layout: invalid\n", want: "gwt:"},
		{name: "command error", arg: "unknown", want: "gwt: unknown command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte(tc.file), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(os.Args[0], "-test.run=TestMain$") // #nosec G204,G702 -- invokes this test binary's fixed helper.
			cmd.Env = append(os.Environ(), "GWT_MAIN_HELPER=1", "GWT_MAIN_DIR="+dir, "GWT_MAIN_ARG="+tc.arg)
			out, err := cmd.CombinedOutput()
			if tc.name == "version" {
				if err != nil || !strings.Contains(string(out), "dev\n") {
					t.Fatalf("output %q, error %v", out, err)
				}
				return
			}
			if err == nil || !strings.Contains(string(out), tc.want) {
				t.Fatalf("output %q, error %v", out, err)
			}
		})
	}
}
