package scanner

import (
	"strings"
	"testing"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func TestFormatUsersReport_ContainsCountsAndSections(t *testing.T) {
	good := []guard.User{{ID: 1, FirstName: "Alice", Username: "alice"}}
	unknown := []guard.User{{ID: 2, FirstName: "Bob"}}
	bad := []guard.User{
		{ID: 3, FirstName: "Eve"},
		{ID: 4, FirstName: "Mallory"},
	}

	out := formatUsersReport("My Channel", good, unknown, bad)

	checks := []string{
		"Users of My Channel",
		"Good <b>1</b>",
		"Unknown <b>1</b>",
		"Bad <b>2</b>",
		"Good (1):",
		"Bad (2):",
		"t.me/alice", // username link present
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\nfull:\n%s", want, out)
		}
	}
}

func TestFormatUsersReport_TruncatesLargeSections(t *testing.T) {
	// 50 bad users — over the per-section cap of 40.
	big := make([]guard.User, 50)
	for i := range big {
		big[i] = guard.User{ID: int64(i), FirstName: "u"}
	}
	out := formatUsersReport("Big", nil, nil, big)

	if !strings.Contains(out, "… and 10 more") {
		t.Errorf("expected truncation marker, got:\n%s", out)
	}
}

func TestFormatUsersReport_OmitsEmptySections(t *testing.T) {
	out := formatUsersReport("Ch", []guard.User{{ID: 1, FirstName: "A"}}, nil, nil)
	if strings.Contains(out, "Unknown (") {
		t.Errorf("empty Unknown section should be omitted:\n%s", out)
	}
	if strings.Contains(out, "Bad (") {
		t.Errorf("empty Bad section should be omitted:\n%s", out)
	}
}

func TestUserLink_Fallbacks(t *testing.T) {
	phone := "+15550000"
	cases := []struct {
		user guard.User
		want string
	}{
		{guard.User{ID: 1, FirstName: "A", Username: "ali"}, `t.me/ali`},
		{guard.User{ID: 2, FirstName: "B", Phone: &phone}, `t.me/+` + phone},
		{guard.User{ID: 3, FirstName: "C"}, `tg://user?id=3`},
	}
	for _, c := range cases {
		got := userLink(c.user)
		if !strings.Contains(got, c.want) {
			t.Errorf("userLink %+v = %q, want it to contain %q", c.user, got, c.want)
		}
	}
}

func TestParseUsersChannelArg(t *testing.T) {
	cases := []struct {
		in     string
		wantID int64
		wantOK bool
	}{
		{"/users 123", 123, true},
		{"/users -100456", -100456, true},
		{"/users abc", 0, false},
		{"/users", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		id, ok := parseUsersChannelArg(c.in)
		if ok != c.wantOK || id != c.wantID {
			t.Errorf("parseUsersChannelArg(%q) = (%d, %v), want (%d, %v)",
				c.in, id, ok, c.wantID, c.wantOK)
		}
	}
}
