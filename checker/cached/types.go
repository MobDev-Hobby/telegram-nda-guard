package cached

import "time"

type Cache struct {
	hasAccess bool
	updated   time.Time
}
