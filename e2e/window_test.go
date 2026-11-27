package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWindowInInsertMode(t *testing.T) {
	// vim-guise opens the file to edit in a callback: the window changes
	// in Insert mode, with no InsertLeave. Leaving the window disables,
	// and the mappings go from the buffer they were set in.
	for _, ed := range []struct{ env, name string }{{"GODA_VIM", "vim"}, {"GODA_NVIM", "nvim"}} {
		t.Run(ed.name, func(t *testing.T) {
			raw := drivePty(t, ed.env, ed.name, [][]any{
				{"i\n", 1.0}, // <C-j>
				{"a", 0.3},
				{dump, 0.2},
				{"\x1bOQ", 1.0}, // <F2>
				{"fix", 0.3},
				{dump, 0.2},
				{"\x1b:bwipe!\r", 0.5}, // back to the first buffer
				{"Als", 0.3},
				{dump, 0.2},
			})
			type rec struct {
				Line    string `json:"line"`
				Enabled bool   `json:"enabled"`
				Mapped  int    `json:"mapped"`
			}
			var got []rec
			for l := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
				var r rec
				if err := json.Unmarshal([]byte(l), &r); err != nil {
					t.Fatalf("bad record %q: %v", l, err)
				}
				got = append(got, r)
			}
			want := []rec{
				{"あ", true, 1},
				{"fix", false, 0},
				{"あls", false, 0},
			}
			if len(got) != len(want) {
				t.Fatalf("records: %+v", got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("record %d: %+v, want %+v", i, got[i], want[i])
				}
			}
		})
	}
}
