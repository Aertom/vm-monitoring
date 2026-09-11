package model

import (
	"reflect"
	"testing"
)

func customSet(t *testing.T) *FamilySet {
	t.Helper()
	s, err := NewFamilySet([]FamilyDef{
		{Name: "sm"},
		{Name: "wks", Match: []string{"wks"}},
		{Name: "ops", Shared: true},
	})
	if err != nil {
		t.Fatalf("NewFamilySet: %v", err)
	}
	return s
}

func TestFamilySet_DetectCustom(t *testing.T) {
	s := customSet(t)
	cases := []struct {
		in   string
		want Family
	}{
		{"app-sm-01", "sm"},
		{"app-WKS-02", "wks"},
		{"ops-relay", "ops"},
		{"ws-legacy-01", FamilyUnknown},
		{"srv01", FamilyUnknown},
	}
	for _, c := range cases {
		if got := s.Detect(c.in); got != c.want {
			t.Errorf("Detect(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestFamilySet_Roles(t *testing.T) {
	s := customSet(t)
	if !s.IsCore("sm") || !s.IsCore("wks") {
		t.Errorf("sm/wks devraient être core")
	}
	if s.IsCore("ops") || !s.IsShared("ops") {
		t.Errorf("ops devrait être shared")
	}
	if s.IsCore("unknown") || s.IsShared("unknown") {
		t.Errorf("unknown ne devrait avoir aucun rôle")
	}
	if !reflect.DeepEqual(s.Names(), []Family{"sm", "wks", "ops"}) {
		t.Errorf("Names=%v", s.Names())
	}
}

func TestFamilySet_Validation(t *testing.T) {
	for name, defs := range map[string][]FamilyDef{
		"nom vide":        {{Name: "  "}},
		"unknown réservé": {{Name: "Unknown"}},
		"doublon":         {{Name: "sm"}, {Name: "SM"}},
	} {
		if _, err := NewFamilySet(defs); err == nil {
			t.Errorf("%s: erreur attendue", name)
		}
	}
	if _, err := NewFamilySet(nil); err != nil {
		t.Errorf("liste vide devrait donner le défaut: %v", err)
	}
}

func TestFamilySet_NilSafe(t *testing.T) {
	var s *FamilySet
	if got := s.Detect("app-sm-01"); got != FamilySM {
		t.Errorf("nil Detect=%q", got)
	}
	if !s.IsCore(FamilySM) || !s.IsShared(FamilyOA) {
		t.Errorf("nil rôles incorrects")
	}
	if len(s.Names()) != 4 {
		t.Errorf("nil Names=%v", s.Names())
	}
}
