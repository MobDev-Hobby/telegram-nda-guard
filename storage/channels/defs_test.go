package channels

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

// Records written before CleanOptions existed must still load, and keep
// using the defaults.
func TestLoadRecordWithoutCleanOptions(t *testing.T) {
	var pc ProtectedChannel
	require.NoError(t, json.Unmarshal(
		[]byte(`{"ID":-1001,"CommandChannelIDs":[5],"AutoScan":true,"AutoClean":false,"AllowClean":true}`), &pc))
	assert.Equal(t, int64(-1001), pc.ID)
	assert.Nil(t, pc.CleanOptions)
}

func TestCleanOptionsRoundTrip(t *testing.T) {
	in := ProtectedChannel{ID: 1, CleanOptions: &processors.CleanOptions{KeepBanned: true}}
	raw, err := json.Marshal(in)
	require.NoError(t, err)

	var out ProtectedChannel
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, in, out)

	raw, err = json.Marshal(ProtectedChannel{ID: 1})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "CleanOptions", "unset options are not written")
}

// Records written before ScanSchedule existed must still load and keep using
// the service default.
func TestLoadRecordWithoutScanSchedule(t *testing.T) {
	var pc ProtectedChannel
	require.NoError(t, json.Unmarshal(
		[]byte(`{"ID":-1001,"CommandChannelIDs":[5],"AutoScan":true}`), &pc))
	assert.Nil(t, pc.ScanSchedule)
}

func TestScanScheduleRoundTrip(t *testing.T) {
	in := ProtectedChannel{ID: 1, ScanSchedule: &processors.ScanSchedule{
		At:       "13:05",
		Weekdays: []time.Weekday{time.Monday, time.Friday},
		TZ:       "Europe/Moscow",
	}}
	raw, err := json.Marshal(in)
	require.NoError(t, err)

	var out ProtectedChannel
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, in, out)

	raw, err = json.Marshal(ProtectedChannel{ID: 1})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "ScanSchedule", "an unset schedule is not written")
}

// The summary release time rides along in NotificationOptions, so records
// without it keep the interval behaviour.
func TestNotificationReportTimeRoundTrip(t *testing.T) {
	var pc ProtectedChannel
	require.NoError(t, json.Unmarshal(
		[]byte(`{"ID":1,"Notifications":{"mode":"all","intervalHours":24}}`), &pc))
	require.NotNil(t, pc.Notifications)
	assert.Empty(t, pc.Notifications.At)

	raw, err := json.Marshal(ProtectedChannel{ID: 1, Notifications: &processors.NotificationOptions{
		Mode: "all", IntervalHours: 24, At: "13:00",
	}})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"at":"13:00"`)
}
