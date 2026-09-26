package channels

import (
	"encoding/json"
	"testing"

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
