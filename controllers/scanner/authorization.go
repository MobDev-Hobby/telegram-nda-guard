package scanner

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// allowAllAuthorizer permits every update. It is the default when no
// Authorizer is configured, preserving the pre-authorization behavior where any
// member of a controlling chat could run commands. Operators who want to
// restrict access should pass a real Authorizer via WithAuthorizer.
type allowAllAuthorizer struct{}

func (allowAllAuthorizer) Authorize(_ context.Context, _ *guard.Update) (bool, error) {
	return true, nil
}

// authorizer returns the configured Authorizer, falling back to an allow-all
// implementation when none was set. This keeps Run() backwards-compatible.
func (d *Domain) resolvedAuthorizer() Authorizer {
	if d.authorizer != nil {
		return d.authorizer
	}
	return allowAllAuthorizer{}
}

// authorize checks whether the sender of update may run a protected command.
// Returns true when authorized (or when authorization is not enforced). On a
// denial it logs at debug level so operators can see why a command was ignored.
func (d *Domain) authorize(ctx context.Context, update *guard.Update) bool {
	ok, err := d.resolvedAuthorizer().Authorize(ctx, update)
	if err != nil {
		d.log.Errorf("authorize denied command: %s", err)
		return false
	}
	if !ok {
		d.log.Debugf("authorize denied command from update")
	}
	return ok
}

// requireAuth wraps a command callback so that it only runs when the sender is
// authorized. It is the single chokepoint used when registering handlers, so
// every protected command goes through authorization uniformly.
func (d *Domain) requireAuth(
	callback func(ctx context.Context, update *guard.Update),
) func(ctx context.Context, update *guard.Update) {
	return func(ctx context.Context, update *guard.Update) {
		if !d.authorize(ctx, update) {
			return
		}
		callback(ctx, update)
	}
}
