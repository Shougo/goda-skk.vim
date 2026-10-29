// Package e2e runs the input method in a real Vim and Neovim: the scenario
// testdata/test.vim, with the binary of testbin.
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// editor is the executable of Vim or Neovim: $GODA_VIM or $GODA_NVIM, or
// the one in PATH.
func editor(t *testing.T, env, name string) string {
	t.Helper()
	if p := os.Getenv(env); p != "" {
		return p
	}
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("no %s", name)
	}
	return p
}

// setup builds the binary and returns the environment of the scenario.
func setup(t *testing.T) []string {
	t.Helper()
	return setupBinary(t, "./testbin")
}

// resultFile is where the scenario writes its result.
func resultFile(env []string) string {
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "GODA_SKK_RESULT="); ok {
			return v
		}
	}
	return ""
}

// setupBinary builds the binary of pkg and returns the environment of a
// scenario.
func setupBinary(t *testing.T, pkg string) []string {
	t.Helper()
	if testing.Short() {
		t.Skip("-short")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "goda-skk")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, pkg).CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	// The Vim script of goda.vim: the module this module requires.
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/Shougo/goda.vim").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	godaDir := strings.TrimSpace(string(out))
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	jisyo, err := filepath.Abs("testdata/SKK-JISYO.test")
	if err != nil {
		t.Fatal(err)
	}
	return append(os.Environ(),
		"GODA_VIM_DIR="+godaDir,
		"GODA_SKK_DIR="+root,
		"GODA_SKK_BINARY="+bin,
		"GODA_SKK_JISYO="+jisyo,
		"GODA_SKK_USER_JISYO="+filepath.Join(dir, "user-jisyo"),
		"GODA_SKK_RESULT="+filepath.Join(dir, "result"),
	)
}

func run(t *testing.T, env []string, name string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	b, rerr := os.ReadFile(resultFile(env))
	if rerr != nil {
		t.Fatalf("no result (%v): %v\n%s", err, rerr, out)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	ok := false
	for _, l := range lines {
		if l == "ok" {
			ok = true
		}
	}
	if !ok {
		t.Errorf("%s", strings.Join(lines, "\n"))
	}
}

func script(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("testdata/test.vim")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVim(t *testing.T) {
	vim := editor(t, "GODA_VIM", "vim")
	env := setup(t)
	run(t, env, vim, "-N", "-u", "NONE", "-i", "NONE", "-n", "--not-a-term",
		"-S", script(t))
}

func TestNeovim(t *testing.T) {
	nvim := editor(t, "GODA_NVIM", "nvim")
	env := setup(t)
	run(t, env, nvim, "--headless", "--clean", "-n", "-S", script(t))
}
