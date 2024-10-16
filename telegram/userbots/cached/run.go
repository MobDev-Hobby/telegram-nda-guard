package cached

import (
	"context"
)

func (d *Domain) Run(
	ctx context.Context,
) error {

	return d.userBot.Run(
		ctx,
	)
}
