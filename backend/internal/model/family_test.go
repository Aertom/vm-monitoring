package model

import "testing"

func TestDetectFamily(t *testing.T) {
	cases := []struct {
		hostname string
		want     Family
	}{
		{"app-SM-prod-01", FamilySM},
		{"app-cm-prod-01", FamilyCM},
		{"WS-prod-02", FamilyWS},
		{"oa-backup-03", FamilyOA},
		{"unknown-host", FamilyUnknown},
		{"", FamilyUnknown},
	}
	for _, c := range cases {
		got := DetectFamily(c.hostname)
		if got != c.want {
			t.Errorf("DetectFamily(%q) = %q, want %q", c.hostname, got, c.want)
		}
	}
}
