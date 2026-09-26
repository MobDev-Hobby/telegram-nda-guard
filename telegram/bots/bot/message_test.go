package bot

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestChatButtonChannel(t *testing.T) {
	raw, err := json.Marshal(requestChatButton(7, true))
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, true, got["chat_is_channel"])

	botRights := got["bot_administrator_rights"].(map[string]any)
	assert.Equal(t, true, botRights["can_restrict_members"])
	assert.Equal(t, false, botRights["can_promote_members"])
	userRights := got["user_administrator_rights"].(map[string]any)
	assert.Equal(t, false, userRights["can_promote_members"])
}

func TestRequestChatButtonGroup(t *testing.T) {
	raw, err := json.Marshal(requestChatButton(8, false))
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, false, got["chat_is_channel"])
}
