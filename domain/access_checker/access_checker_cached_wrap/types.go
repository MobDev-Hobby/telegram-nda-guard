package access_checker_cached_wrap

import "time"

type Cache struct {
	hasAccess bool
	updated time.Time
}
