package redis

import (
	"encoding/hex"
	"fmt"
)

func (s *Domain) getKeyName(name string) string {
	return fmt.Sprintf(
		"%s:%s",
		s.keyPrefix,
		hex.EncodeToString([]byte(name)),
	)
}
