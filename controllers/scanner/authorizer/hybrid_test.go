package authorizer

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// stubAdminLister is a controllable ChatAdminLister.
type stubAdminLister struct {
	admins []int64
	err    error
	calls  int
}

func (s *stubAdminLister) GetChatAdministrators(_ context.Context, _ int64) ([]int64, error) {
	s.calls++
	return s.admins, s.err
}

func msgUpdate(chatID, userID int64) *guard.Update {
	return &guard.Update{
		Message: &guard.MessageReceived{
			Message: guard.Message{ChatID: chatID},
			User:    guard.User{ID: userID},
		},
	}
}

func callbackUpdate(chatID, userID int64) *guard.Update {
	return &guard.Update{
		CallbackQuery: &guard.CallbackQuery{
			Data: "/scan 1",
			Message: &guard.MessageReceived{
				Message: guard.Message{ChatID: chatID},
				User:    guard.User{ID: userID},
			},
		},
	}
}

func TestAuthorize_OwnerAlwaysAllowed(t *testing.T) {
	a := New(&stubAdminLister{}, WithOwner(42))
	ok, err := a.Authorize(context.Background(), msgUpdate(1, 42))
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestAuthorize_Allowlist(t *testing.T) {
	a := New(&stubAdminLister{}, WithAllowlist([]int64{7, 8}))
	ok, err := a.Authorize(context.Background(), msgUpdate(1, 8))
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = a.Authorize(context.Background(), msgUpdate(1, 99))
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestAuthorize_AdminCheck(t *testing.T) {
	lst := &stubAdminLister{admins: []int64{100, 101}}
	a := New(lst, WithRequireAdmin())

	ok, err := a.Authorize(context.Background(), msgUpdate(5, 101))
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1, lst.calls, "admin list should be fetched once")
}

func TestAuthorize_AdminCheckDeniesNonAdmin(t *testing.T) {
	lst := &stubAdminLister{admins: []int64{100}}
	a := New(lst, WithRequireAdmin())

	ok, err := a.Authorize(context.Background(), msgUpdate(5, 999))
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestAuthorize_AdminErrorDeniesWithoutPropagating(t *testing.T) {
	lst := &stubAdminLister{err: errors.New("telegram down")}
	a := New(lst, WithRequireAdmin())

	ok, err := a.Authorize(context.Background(), msgUpdate(5, 100))
	// An admin-listing failure must deny (fail-closed) but not error out.
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestAuthorize_NoConfigDeniesAll(t *testing.T) {
	a := New(&stubAdminLister{})
	ok, err := a.Authorize(context.Background(), msgUpdate(1, 1))
	assert.NoError(t, err)
	assert.False(t, ok, "with no owner/allowlist/admin, everything is denied")
}

func TestAuthorize_CallbackUsesMessageSender(t *testing.T) {
	a := New(&stubAdminLister{}, WithOwner(42))
	ok, err := a.Authorize(context.Background(), callbackUpdate(1, 42))
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestAuthorize_OwnerShortCircuitsAdminCheck(t *testing.T) {
	lst := &stubAdminLister{}
	a := New(lst, WithOwner(42), WithRequireAdmin())

	ok, err := a.Authorize(context.Background(), msgUpdate(5, 42))
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 0, lst.calls, "owner must short-circuit before the admin API call")
}

func TestAuthorize_NilUpdateDenied(t *testing.T) {
	a := New(&stubAdminLister{}, WithOwner(42))
	ok, err := a.Authorize(context.Background(), nil)
	assert.Error(t, err)
	assert.False(t, ok)
}
