package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The completion of goda-cmp.vim with the sources of goda-skk.vim
// (cmp/source), typed in a pseudo terminal.
func TestCompletion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no pseudo terminal")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 drives the pseudo terminal")
	}
	for _, ed := range []struct{ env, name string }{{"GODA_VIM", "vim"}, {"GODA_NVIM", "nvim"}} {
		t.Run(ed.name, func(t *testing.T) {
			bin := editor(t, ed.env, ed.name)
			env := setupBinary(t, "./cmpbin")
			out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/Shougo/goda-cmp.vim").Output()
			if err != nil {
				t.Fatalf("go list: %v", err)
			}
			env = append(env, "GODA_CMP_DIR="+strings.TrimSpace(string(out)))
			vimrc, err := filepath.Abs("testdata/cmp.vim")
			if err != nil {
				t.Fatal(err)
			}
			const dump = "\x1d" // Ctrl-]
			steps := [][]any{
				{"i\n", 1.0},      // <C-j>
				{"Ni", 1.0},       // ▽に
				{dump, 0.2},       // にほん: 日本 二本
				{"\x0e\x19", 0.5}, // <C-n> <C-y>
				{dump, 0.2},
				{"\x1a", 0.5}, // <C-z>: kakuteiUndo
				{dump, 0.2},
				{" \n", 0.3}, // 二本
				{"Okuri", 1.0},
				{dump, 0.2}, // 送り 贈り
				{"\x0e\x0e\x19", 0.5},
				{dump, 0.2},
			}
			b, err := json.Marshal(steps)
			if err != nil {
				t.Fatal(err)
			}
			drive, err := filepath.Abs("testdata/drive.py")
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("python3", "-I", drive, bin, vimrc, string(b))
			cmd.Env = env
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("driver: %v\n%s", err, out)
			}
			raw, err := os.ReadFile(resultFile(env))
			if err != nil {
				t.Fatalf("nothing recorded: %v", err)
			}
			type rec struct {
				Line  string   `json:"line"`
				Words []string `json:"words"`
				Phase string   `json:"phase"`
			}
			var recs []rec
			for l := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
				var r rec
				if err := json.Unmarshal([]byte(l), &r); err != nil {
					t.Fatalf("bad record %q: %v", l, err)
				}
				recs = append(recs, r)
			}
			want := []rec{
				{"▽に", []string{"日本", "二本"}, "input:okurinasi"},
				{"日本", nil, "input"},
				{"▼日本", nil, "henkan"},
				{"二本▽おくり", []string{"送り", "贈り"}, "input:okurinasi"},
				{"二本贈り", nil, "input"},
			}
			if len(recs) != len(want) {
				t.Fatalf("records: %+v", recs)
			}
			for i, r := range recs {
				w := want[i]
				if r.Line != w.Line || !slices.Equal(r.Words, w.Words) || r.Phase != w.Phase {
					t.Errorf("record %d: %+v, want %+v", i, r, w)
				}
			}
		})
	}
}
