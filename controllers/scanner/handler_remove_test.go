package scanner

import (
	"testing"
)

func TestParseRemoveChannelArg(t *testing.T) {
	cases := []struct {
		in     string
		wantID int64
		wantOK bool
	}{
		{"/remove 100", 100, true},
		{"/rmconfirm -100200", -100200, true},
		{"/remove abc", 0, false},
		{"/remove", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		id, ok := parseRemoveChannelArg(c.in)
		if ok != c.wantOK || id != c.wantID {
			t.Errorf("parseRemoveChannelArg(%q) = (%d, %v), want (%d, %v)",
				c.in, id, ok, c.wantID, c.wantOK)
		}
	}
}

func TestChannelTitleForRemove_NotControlled(t *testing.T) {
	d := &Domain{
		commandChannels: map[int64][]int64{
			5:  {100, 101},
			10: {200},
		},
		channels: map[int64]ChannelInfo{
			100: {id: 100, title: "Alpha"},
		},
	}

	// Channel 100 is controlled from chat 5, not chat 9.
	if _, ok := d.channelTitleForRemove(100, 9); ok {
		t.Error("channel 100 must not be removable from chat 9 (not controlled)")
	}
}

func TestChannelTitleForRemove_ControlledWithCache(t *testing.T) {
	d := &Domain{
		commandChannels: map[int64][]int64{5: {100}},
		channels:        map[int64]ChannelInfo{100: {id: 100, title: "Alpha"}},
	}

	title, ok := d.channelTitleForRemove(100, 5)
	if !ok {
		t.Fatal("expected channel 100 to be controllable from chat 5")
	}
	if title != "Alpha" {
		t.Errorf("title = %q, want Alpha", title)
	}
}

func TestChannelTitleForRemove_ControlledWithoutCache(t *testing.T) {
	// Channel is controlled but not yet in the channels cache (e.g. before the
	// first GetChat). Should still return a usable (id-based) title.
	d := &Domain{
		commandChannels: map[int64][]int64{5: {777}},
		channels:        map[int64]ChannelInfo{},
	}
	title, ok := d.channelTitleForRemove(777, 5)
	if !ok {
		t.Fatal("expected controllable")
	}
	if title != "777" {
		t.Errorf("title = %q, want 777", title)
	}
}
