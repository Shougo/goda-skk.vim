package skk

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Shougo/goda.vim/sdk"
)

// memDict is a dictionary in memory.
type memDict struct {
	ari, nasi map[string][]string
}

func (d memDict) Lookup(_ context.Context, typ HenkanType, word string) ([]string, error) {
	if typ == OkuriAri {
		return d.ari[word], nil
	}
	return d.nasi[word], nil
}

func (d memDict) Complete(_ context.Context, prefix string) ([]Entry, error) {
	var out []Entry
	for w, cs := range d.nasi {
		if strings.HasPrefix(w, prefix) {
			out = append(out, Entry{w, cs})
		}
	}
	return out, nil
}

var testDict = memDict{
	ari: map[string][]string{
		"おくr":  {"送"},
		"えらn":  {"選"},
		"うたがt": {"疑"},
		"うたがc": {"疑"},
		"はg":   {"剥"},
	},
	nasi: map[string][]string{
		"へんかん":   {"変換", "返還"},
		"かんじ":    {"漢字", "感じ", "幹事", "監事", "完治", "寛治", "莞爾", "冠辞", "環時"},
		"ちゅうしゃく": {"注釈;これは注釈です"},
		"file":   {"ファイル"},
		"#ねん":    {"#1年", "#3年"},
		"ちょう>":   {"超"},
		">てき":    {"的"},
	},
}

// sim is an editor: the line the keys go to.
type sim struct {
	t    *testing.T
	p    *Plugin
	line string
	mode string // "i" or "c"
	on   bool
	res  Result
	// confirm is the answer to confirm().
	confirm int
	// after is the text after the cursor.
	after string
	// register answers the prompt of registerWord.
	register func(prompt string) string
}

func newSim(t *testing.T, cfg Config) *sim {
	t.Helper()
	if cfg.Dictionaries == nil {
		cfg.Dictionaries = []Dictionary{testDict}
	}
	if cfg.UserDictionary == "" {
		cfg.UserDictionary = filepath.Join(t.TempDir(), "jisyo")
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := &sim{t: t, p: p, mode: "i"}
	s.call("enable")
	return s
}

func (s *sim) Meta() sdk.Meta { return sdk.Meta{} }
func (s *sim) Call(_ context.Context, fn string, args ...any) (any, error) {
	switch fn {
	case "confirm":
		return int64(s.confirm), nil
	case "goda_skk#_locate_kakutei":
		// As Vim does it: the line has to start with the conversion.
		prefix, prev, lnum := s.cursorLine()
		if args[1].(int) != lnum {
			return false, nil
		}
		head := args[2].(string) + args[3].(string)
		text := prev + s.after
		if !strings.HasPrefix(text, head) {
			return false, nil
		}
		s.line, s.after = prefix+head, text[len(head):]
		return true, nil
	}
	return nil, nil
}

// cursorLine is the line of the cursor: the lines before it, the text
// before the cursor, and its number.
func (s *sim) cursorLine() (prefix, prev string, lnum int) {
	i := strings.LastIndexByte(s.line, '\n')
	return s.line[:i+1], s.line[i+1:], strings.Count(s.line, "\n") + 1
}

// left moves the cursor n characters to the left.
func (s *sim) left(n int) {
	for range n {
		_, size := utf8.DecodeLastRuneInString(s.line)
		s.after = s.line[len(s.line)-size:] + s.after
		s.line = s.line[:len(s.line)-size]
	}
}
func (s *sim) Notify(context.Context, string, ...any) error { return nil }
func (s *sim) Eval(context.Context, string) (any, error)    { return nil, nil }
func (s *sim) Cmd(context.Context, string) error            { return nil }

// call runs fn for keys and applies the result to the line.
func (s *sim) call(fn string, keys ...string) Result {
	s.t.Helper()
	_, prev, lnum := s.cursorLine()
	s.p.mu.Lock()
	res, err := s.p.run(context.Background(), s, fn, keys, Status{
		Mode: s.mode, PrevInput: prev, Bufnr: 1, Lnum: lnum, Selected: -1,
	}, s.on)
	s.p.mu.Unlock()
	if err != nil {
		s.t.Fatal(err)
	}
	s.apply(res.Keys)
	if r := res.Restore; r != nil {
		prefix, prev, lnum := s.cursorLine()
		if r.Lnum == lnum {
			text := prev + s.after
			s.line, s.after = prefix+text[:r.Col-1], text[r.Col-1:]
		}
	}
	if !strings.HasSuffix(s.line, res.PreEdit) {
		s.t.Errorf("pre-edit %q is not before the cursor: %q", res.PreEdit, s.line)
	}
	if res.Enable {
		s.on = true
	}
	if res.Disable {
		s.on = false
	}
	s.res = res
	if res.Register != "" && fn != "registerDone" {
		// As Vim does: ask, and tell.
		word := ""
		if s.register != nil {
			word = s.register(res.Register)
		}
		return s.call("registerDone", word)
	}
	return res
}

// apply types keys into the line: a backspace deletes a character, <C-g>u
// (the undo point) and <Esc> are not text.
func (s *sim) apply(keys string) {
	keys = strings.ReplaceAll(keys, "\x07u", "")
	for _, r := range keys {
		switch r {
		case '\b':
			_, n := utf8.DecodeLastRuneInString(s.line)
			s.line = s.line[:len(s.line)-n]
		case '\x1b':
		default:
			s.line += string(r)
		}
	}
}

// typ types the characters of keys, one key each (" " is <Space>), or
// the key notations given as "<...>".
func (s *sim) typ(keys string) {
	s.t.Helper()
	for keys != "" {
		k := keys[:1]
		if keys[0] == '<' {
			if i := strings.IndexByte(keys, '>'); i > 0 {
				k = keys[:i+1]
			}
		} else {
			_, n := utf8.DecodeRuneInString(keys)
			k = keys[:n]
		}
		keys = keys[len(k):]
		if k == " " {
			k = "<space>"
		}
		s.call("handleKey", k)
	}
}

func (s *sim) preedit() string { return s.p.c.text() }

func TestKanaInput(t *testing.T) {
	for _, tc := range []struct{ keys, want string }{
		{"nihongoutteiki", "にほんごうっていき"},
		{"n n n n ", "ん ん ん ん "},
		{"@a", "@あ"},
		{" na", " な"},
		{"kka", "っか"},
		{"z-z.", "〜…"},
	} {
		s := newSim(t, Config{})
		s.typ(tc.keys)
		if s.line != tc.want {
			t.Errorf("%q: %q, want %q", tc.keys, s.line, tc.want)
		}
	}
}

func TestHenkanPoint(t *testing.T) {
	s := newSim(t, Config{})
	for _, tc := range [][2]string{
		{";", "▽"}, {";", "▽"}, {"y", "▽y"}, {"a", "▽や"}, {";", "▽や*"},
		{";", "▽や*"}, {"t", "▽や*t"}, {"t", "▽や*っt"},
		// ";" is no kana after "t": the feed is dropped.
		{";", "▽や*っ"},
	} {
		s.typ(tc[0])
		if got := s.preedit(); got != tc[1] {
			t.Fatalf("after %q: %q, want %q", tc[0], got, tc[1])
		}
		if s.line != tc[1] {
			t.Fatalf("after %q: the line is %q, want %q", tc[0], s.line, tc[1])
		}
	}
}

func TestUpperCase(t *testing.T) {
	for _, tc := range []struct{ keys, want string }{
		{"HogeP", "▽ほげ*p"},
		{"sAsS", "▽さっ*s"},
	} {
		s := newSim(t, Config{})
		s.typ(tc.keys)
		if got := s.preedit(); got != tc.want {
			t.Errorf("%q: %q, want %q", tc.keys, got, tc.want)
		}
	}
	s := newSim(t, Config{LowercaseMap: map[string]string{"+": "a"}})
	s.typ("+")
	if got := s.preedit(); got != "▽あ" {
		t.Errorf("lowercaseMap: %q", got)
	}
}

func TestDeleteChar(t *testing.T) {
	s := newSim(t, Config{})
	s.typ(";ya;tt")
	want := s.preedit()
	for want != "" {
		s.typ("<bs>")
		want = dropLastRune(want)
		if got := s.preedit(); got != want {
			t.Fatalf("%q, want %q", got, want)
		}
	}
	// In direct input, a backspace for the buffer.
	s.typ("a<bs>")
	if s.line != "" {
		t.Errorf("line %q", s.line)
	}
}

func TestOkuriNasiHenkan(t *testing.T) {
	s := newSim(t, Config{})
	s.typ(";henkan ")
	if got := s.preedit(); got != "▼変換" {
		t.Fatalf("%q", got)
	}
	s.typ(" ")
	if got := s.preedit(); got != "▼返還" {
		t.Fatalf("%q", got)
	}
	s.typ("x")
	if got := s.preedit(); got != "▼変換" {
		t.Fatalf("%q", got)
	}
	s.typ("x")
	if got := s.preedit(); got != "▽へんかん" {
		t.Fatalf("back to the reading: %q", got)
	}
	s.typ(" <c-j>")
	if s.line != "変換" {
		t.Fatalf("line %q", s.line)
	}
	// Learnt: 変換 first, then 返還 after confirming it.
	s.typ("Henkan  <c-j>")
	s.line = ""
	s.typ("Henkan ")
	if got := s.preedit(); got != "▼返還" {
		t.Errorf("after learning: %q", got)
	}
}

func TestOkuriAriHenkan(t *testing.T) {
	for _, tc := range []struct{ keys, want string }{
		{";oku;ri", "▼送り"},
		{"OkuRi", "▼送り"},
		{";era;nde", "▼選んで"},
		{";utaga;tte", "▼疑って"},
		{";utaga;ccha", "▼疑っちゃ"},
	} {
		s := newSim(t, Config{})
		s.typ(tc.keys)
		if got := s.preedit(); got != tc.want {
			t.Errorf("%q: %q, want %q", tc.keys, got, tc.want)
		}
	}
}

// A vowel after a candidate confirms it, and is typed.
func TestKakuteiByTyping(t *testing.T) {
	s := newSim(t, Config{})
	s.typ("Kanji a")
	if s.line != "漢字あ" || s.preedit() != "" {
		t.Errorf("line %q pre-edit %q", s.line, s.preedit())
	}
}

func TestCandidateList(t *testing.T) {
	s := newSim(t, Config{})
	s.typ("Kanji     ")
	// Four one by one, then the list of the next seven.
	if got := s.preedit(); got != "▼完治" {
		t.Fatalf("%q", got)
	}
	want := []string{"a: 完治", "s: 寛治", "d: 莞爾", "f: 冠辞", "j: 環時"}
	if !slices.Equal(s.res.Popup, want) {
		t.Fatalf("popup %q", s.res.Popup)
	}
	s.typ("s")
	if s.line != "寛治" {
		t.Errorf("chosen: %q", s.line)
	}
	// Past the end: no more pages. (A new user dictionary: 寛治 was
	// learnt.)
	s = newSim(t, Config{})
	s.typ("Kanji      ")
	if got := s.preedit(); got != "▼完治" {
		t.Errorf("past the end: %q", got)
	}
	s.typ("x")
	if got := s.preedit(); got != "▼監事" {
		t.Errorf("back from the list: %q", got)
	}
}

func TestAnnotationAndAffix(t *testing.T) {
	s := newSim(t, Config{})
	s.typ(";tyuusyaku <c-j>")
	if s.line != "注釈" {
		t.Errorf("annotation: %q", s.line)
	}
	s.line = ""
	s.typ("Tyou> ")
	if got := s.preedit(); got != "▼超" {
		t.Errorf("prefix: %q", got)
	}
	s.typ(">teki ")
	if s.line != "超▼的" || s.preedit() != "▼的" {
		t.Errorf("suffix: line %q pre-edit %q", s.line, s.preedit())
	}
}

func TestNumber(t *testing.T) {
	s := newSim(t, Config{})
	s.typ(";2026nenn ")
	if got := s.preedit(); got != "▼２０２６年" {
		t.Errorf("%q", got)
	}
	s.typ(" ")
	if got := s.preedit(); got != "▼二千二十六年" {
		t.Errorf("%q", got)
	}
}

func TestModes(t *testing.T) {
	s := newSim(t, Config{})
	s.typ("q")
	if s.res.Mode != ModeKata {
		t.Fatalf("mode %q", s.res.Mode)
	}
	s.typ("kana")
	if s.line != "カナ" {
		t.Errorf("katakana: %q", s.line)
	}
	s.typ("q")
	s.line = ""
	// A reading in katakana.
	s.typ("Kanaq")
	if s.line != "カナ" || s.res.Mode != ModeHira {
		t.Errorf("reading to katakana: %q (%s)", s.line, s.res.Mode)
	}
	s.line = ""
	s.typ("<c-q>kana")
	if s.line != "ｶﾅ" || s.res.Mode != ModeHankata {
		t.Errorf("hankata: %q (%s)", s.line, s.res.Mode)
	}
	s.typ("<c-q>")
	s.line = ""
	s.typ("Lab")
	if s.line != "ａｂ" || s.res.Mode != ModeZenkaku {
		t.Errorf("zenkaku: %q (%s)", s.line, s.res.Mode)
	}
	s.typ("<c-j>")
	if s.res.Mode != ModeHira {
		t.Errorf("<C-j> back to hiragana: %s", s.res.Mode)
	}
	s.line = ""
	s.typ("/file ")
	if got := s.preedit(); got != "▼ファイル" || s.res.Mode != ModeAbbrev {
		t.Errorf("abbrev: %q (%s)", got, s.res.Mode)
	}
	s.typ("<c-j>")
	if s.line != "ファイル" || s.res.Mode != ModeHira {
		t.Errorf("abbrev confirmed: %q (%s)", s.line, s.res.Mode)
	}
	// Okurigana follow the mode.
	s.line = ""
	s.typ("q;ha;ge<c-j>")
	if s.line != "剥ゲ" {
		t.Errorf("okurigana in katakana: %q", s.line)
	}
}

func TestKakuteiKeyLeavesMode(t *testing.T) {
	s := newSim(t, Config{})
	s.typ("qk<c-j>")
	if s.res.Mode != ModeKata {
		t.Errorf("%s", s.res.Mode)
	}
	s.typ("<c-j>")
	if s.res.Mode != ModeHira {
		t.Errorf("%s", s.res.Mode)
	}
}

func TestCancel(t *testing.T) {
	s := newSim(t, Config{})
	s.typ("A<c-g>")
	if s.preedit() != "" || s.line != "" {
		t.Errorf("%q %q", s.preedit(), s.line)
	}
	s.typ("A <c-g>")
	if s.preedit() != "" {
		t.Errorf("%q", s.preedit())
	}
	s = newSim(t, Config{ImmediatelyCancel: new(false)})
	s.typ("Henkan <c-g>")
	if got := s.preedit(); got != "▽へんかん" {
		t.Errorf("not immediately: %q", got)
	}
	s.typ("<c-g>")
	if s.preedit() != "" {
		t.Errorf("%q", s.preedit())
	}
}

func TestNewline(t *testing.T) {
	s := newSim(t, Config{})
	s.typ("Henkan <cr>")
	if s.line != "変換\n" {
		t.Errorf("%q", s.line)
	}
	s = newSim(t, Config{EggLikeNewline: true})
	s.typ("Henkan <cr>")
	if s.line != "変換" {
		t.Errorf("egg-like: %q", s.line)
	}
}

func TestUndoPoint(t *testing.T) {
	s := newSim(t, Config{})
	res := s.call("handleKey", "K")
	res2 := s.call("handleKey", "a")
	_ = res2
	if !strings.Contains(res.Keys, "\x07u") {
		t.Errorf("no undo point at the start of a reading: %q", res.Keys)
	}
	s.mode = "c"
	s.line = ""
	s.p.c = s.p.newContext()
	if res := s.call("handleKey", "K"); strings.Contains(res.Keys, "\x07u") {
		t.Errorf("undo point on the command line: %q", res.Keys)
	}
}

func TestEscapeAndDisable(t *testing.T) {
	s := newSim(t, Config{})
	s.typ("Henkan ")
	res := s.call("handleKey", "<esc>")
	if s.line != "変換" || !strings.HasSuffix(res.Keys, "\x1b") || !res.Disable || res.Mode != "" {
		t.Errorf("escape: line %q keys %q %+v", s.line, res.Keys, res)
	}
	s.call("enable")
	s.typ("Kanji")
	res = s.call("disable")
	if s.line != "変換かんじ" || !res.Disable {
		t.Errorf("disable: %q %+v", s.line, res)
	}
	s.call("enable")
	res = s.call("handleKey", "l")
	if !res.Disable {
		t.Errorf("l: %+v", res)
	}
	// The enabling key confirms what is typed, a romaji too.
	s.call("enable")
	s.typ("nihonn")
	s.call("enable", "<c-j>")
	s.typ("n")
	if res := s.call("enable", "<c-j>"); s.line != "変換かんじにほんん" || res.Disable {
		t.Errorf("enable: %q %+v", s.line, res)
	}
	s.call("disable")
	// Toggle.
	if res := s.call("toggle"); !res.Enable {
		t.Errorf("toggle on: %+v", res)
	}
	if res := s.call("toggle"); !res.Disable {
		t.Errorf("toggle off: %+v", res)
	}
}

func TestKeepMode(t *testing.T) {
	s := newSim(t, Config{KeepMode: true})
	s.typ("q")
	s.call("disable")
	if res := s.call("enable"); res.Mode != ModeKata {
		t.Errorf("kept: %q", res.Mode)
	}
	s = newSim(t, Config{})
	s.typ("q")
	s.call("disable")
	if res := s.call("enable"); res.Mode != ModeHira {
		t.Errorf("not kept: %q", res.Mode)
	}
}

// When the line does not end with the pre-edit (a completion changed it),
// the input starts over.
func TestMismatch(t *testing.T) {
	s := newSim(t, Config{})
	s.typ("Kan")
	s.line = "something else"
	s.typ("a")
	if s.line != "something elseあ" || s.preedit() != "" {
		t.Errorf("line %q pre-edit %q", s.line, s.preedit())
	}
}

func TestPurge(t *testing.T) {
	s := newSim(t, Config{})
	lookup := func() []string {
		got, _ := s.p.lib.user.Lookup(context.Background(), OkuriNasi, "へんかん")
		return got
	}
	s.typ("Henkan  <c-j>")
	if got := lookup(); !slices.Equal(got, []string{"返還"}) {
		t.Fatalf("learnt %q", got)
	}
	s.line = ""
	s.typ("Henkan ")
	s.confirm = 2
	s.typ("X")
	if got := lookup(); len(got) != 1 {
		t.Errorf("purged without asking: %q", got)
	}
	s.confirm = 1
	s.typ("X")
	if got := lookup(); len(got) != 0 {
		t.Errorf("not purged: %q", got)
	}
	if s.preedit() != "" {
		t.Errorf("pre-edit %q", s.preedit())
	}
}

func TestKeymaps(t *testing.T) {
	if _, err := New(Config{Keymaps: map[string]map[string]string{"input": {"<c-l>": "nope"}}}); err == nil {
		t.Error("an unknown function was accepted")
	}
	s := newSim(t, Config{Keymaps: map[string]map[string]string{"henkan": {"<C-N>": "henkanForward", "x": ""}}})
	s.typ("Henkan <c-n>")
	if got := s.preedit(); got != "▼返還" {
		t.Errorf("<c-n>: %q", got)
	}
	s.typ("x")
	if s.line != "返還x" {
		t.Errorf("x unmapped: %q", s.line)
	}
}

func TestUserDictionaryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jisyo")
	u := NewUserDictionary(path)
	u.Register(OkuriNasi, "かんじ", "漢字")
	u.Register(OkuriNasi, "かんじ", "感じ")
	u.Register(OkuriAri, "おくr", "送")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := okuriAriMarker + "\nおくr /送/\n" + okuriNasiMarker + "\nかんじ /感じ/漢字/\n"
	if string(b) != want {
		t.Errorf("file:\n%s\nwant:\n%s", b, want)
	}
	// Another editor changed it: read again.
	if err := os.WriteFile(path, []byte(okuriNasiMarker+"\nかんじ /幹事/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	future := u.modTime.Add(2e9)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	u.reload()
	if got, _ := u.Lookup(context.Background(), OkuriNasi, "かんじ"); !slices.Equal(got, []string{"幹事"}) {
		t.Errorf("reloaded: %q", got)
	}
}

func TestKakuteiUndo(t *testing.T) {
	s := newSim(t, Config{Keymaps: map[string]map[string]string{"input": {"<C-z>": "kakuteiUndo"}}})
	if !slices.Contains(s.res.MappedKeys, "<c-z>") {
		t.Errorf("mapped keys: %v", s.res.MappedKeys)
	}
	// Right after the conversion.
	s.typ("Kanji <c-j><c-z>")
	if s.line != "▼漢字" || s.res.Phase != "henkan" {
		t.Errorf("undo: %q %+v", s.line, s.res)
	}
	s.typ(" <c-j>")
	if s.line != "感じ" {
		t.Errorf("again: %q", s.line)
	}
	// After typing on: back to the conversion, and the cursor back after
	// choosing another.
	s.typ("aiu")
	s.typ("<c-z>")
	if s.line != "▼感じ" || s.after != "あいう" {
		t.Errorf("walk back: %q|%q", s.line, s.after)
	}
	s.typ("x<c-j>")
	if s.line != "漢字あいう" || s.after != "" {
		t.Errorf("restore: %q|%q", s.line, s.after)
	}
	// Okurigana; and a cursor inside the line stays where it is.
	s.typ("OkuRi")
	s.typ("<c-j>a")
	s.left(2)
	s.typ("<c-z>")
	if s.line != "漢字あいう▼送り" || s.after != "あ" {
		t.Errorf("okuri: %q|%q", s.line, s.after)
	}
	s.typ("<c-g>")
	s.after = ""
	// Nothing to take back: the line changed.
	s.typ("<c-z>")
	if s.line != "漢字あいう" || s.res.Phase != "input" {
		t.Errorf("nothing: %q %+v", s.line, s.res)
	}
	// A new line: not any more.
	s.typ("Kanji <cr>")
	s.typ("<c-z>")
	if s.line != "漢字あいう漢字\n" {
		t.Errorf("newline: %q", s.line)
	}
	// The candidate list again.
	s.typ("Kanji     a")
	s.typ("<c-z>")
	if s.line != "漢字あいう漢字\n▼完治" || len(s.res.Popup) == 0 {
		t.Errorf("popup: %q %v", s.line, s.res.Popup)
	}
}

func TestMarkersAndRules(t *testing.T) {
	s := newSim(t, Config{
		MarkerHenkan:       new(""),
		MarkerHenkanSelect: new(""),
		KanaTable: RomTable.With(
			Rule{From: "jj", Func: "escape"},
			Rule{From: "~", Kana: "〜"},
		),
	})
	s.typ("Kanji")
	if s.line != "かんじ" || s.res.PreEdit != "かんじ" {
		t.Errorf("marker: %q %+v", s.line, s.res)
	}
	s.typ(" ")
	if s.line != "漢字" {
		t.Errorf("select marker: %q", s.line)
	}
	s.typ("<c-j>~ja")
	if s.line != "漢字〜じゃ" {
		t.Errorf("rules: %q", s.line)
	}
	res := s.call("handleKey", "j")
	res = s.call("handleKey", "j")
	if !res.Disable || !strings.HasSuffix(res.Keys, "\x1b") {
		t.Errorf("jj: %+v", res)
	}
	if !slices.Contains(s.p.mappedKeys, "~") || !slices.Contains(s.p.mappedKeys, "<lt>") ||
		!slices.Contains(s.p.mappedKeys, "L") || !slices.Contains(s.p.mappedKeys, "<Space>") {
		t.Errorf("mapped keys: %v", s.p.mappedKeys)
	}
}

func TestRegisterWord(t *testing.T) {
	s := newSim(t, Config{})
	// The prompt has an editor of its own: what is typed there is handled
	// meanwhile.
	s.register = func(prompt string) string {
		if prompt != "[辞書登録] みかん: " {
			t.Errorf("prompt: %q", prompt)
		}
		inner := &sim{t: t, p: s.p, mode: "c"}
		inner.call("enable")
		inner.typ("Kanji <c-j>")
		inner.call("disable")
		return inner.line
	}
	s.typ("Mikan ")
	if s.line != "漢字" || s.res.Phase != "input" {
		t.Errorf("register: %q %+v", s.line, s.res)
	}
	s.register = nil
	s.typ("Mikan ")
	if s.line != "漢字▼漢字" {
		t.Errorf("learned: %q", s.line)
	}
	// Past the last candidate, and cancelled: back to it.
	s.typ(" ")
	if s.line != "漢字▼漢字" {
		t.Errorf("cancelled: %q", s.line)
	}
	s.typ("<c-g>")
	// Okurigana.
	s.register = func(prompt string) string {
		if prompt != "[辞書登録] たべ*る: " {
			t.Errorf("prompt: %q", prompt)
		}
		return "食"
	}
	s.typ("TabeRu")
	if s.line != "漢字食る" {
		t.Errorf("okuri: %q", s.line)
	}
}
