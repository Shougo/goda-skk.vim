package engine

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/Shougo/goda.vim/sdk"
)

// What a binary serves says its version.
var _ = []sdk.ScriptVersioner{(*Plugin)(nil)}

// The Vim script says the version the binary does (sdk.ScriptVersioner).
func TestScriptVersion(t *testing.T) {
	b, err := os.ReadFile("../autoload/goda_skk.vim")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^let s:version = #\{version: (\d+),`).FindSubmatch(b)
	if m == nil {
		t.Fatal("no s:version in autoload/goda_skk.vim")
	}
	if v, _ := strconv.Atoi(string(m[1])); v != ScriptVersion {
		t.Errorf("s:version of autoload/goda_skk.vim is %d, ScriptVersion is %d", v, ScriptVersion)
	}
}
