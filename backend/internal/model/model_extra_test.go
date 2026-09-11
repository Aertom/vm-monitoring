package model

import "testing"

func TestIsInUse(t *testing.T) {
	var g Group
	if g.IsInUse() {
		t.Errorf("groupe vide ne devrait pas être in use")
	}
	g.InUseBy = "alice"
	if !g.IsInUse() {
		t.Errorf("groupe avec InUseBy devrait être in use")
	}
}

func TestDetectFamilyEdge(t *testing.T) {
	cases := []struct {
		in   string
		want Family
	}{
		{"SM", FamilySM},
		{"  cm-01  ", FamilyCM},
		{"sm-cm-01", FamilySM},
		{"srv01", FamilyUnknown},
	}
	for _, c := range cases {
		if got := DetectFamily(c.in); got != c.want {
			t.Errorf("DetectFamily(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
