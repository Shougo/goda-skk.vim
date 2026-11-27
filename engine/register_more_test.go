package engine

import "testing"

func TestRegisterNoCompletion(t *testing.T) {
	// The completion would type into the buffer, under the registration.
	s := newSim(t, Config{})
	s.inline = true
	s.typ("Mikan Kan")
	if _, ok := s.p.Reading(); ok {
		t.Error("a reading to complete while registering")
	}
	s.typ("<c-g><c-g>a")
	if _, ok := s.p.Reading(); !ok {
		t.Error("no reading to complete")
	}
}

func TestRegisterStale(t *testing.T) {
	// Something typed that is not a mapped key (pasted): given up, the
	// reading left.
	s := newSim(t, Config{})
	s.inline = true
	s.typ("Mikan ab")
	s.line += "貼"
	res := s.call("handleKey", "c")
	if res.Registering || s.p.c.registering() {
		t.Errorf("still registering: %+v", res)
	}
	if res.StaleText != "みかん" {
		t.Errorf("stale text %q", res.StaleText)
	}
}
