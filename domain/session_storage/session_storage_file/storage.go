package session_storage_file

import (
	"encoding/hex"
	"fmt"
	"os"
)

type Storage struct {
	name           string
	val            []byte
	sessionStorage *Domain
}

func (s *Storage) getFileName() string {
	return fmt.Sprintf(
		"%s%csession_%s.dat",
		s.sessionStorage.dir,
		os.PathSeparator,
		hex.EncodeToString([]byte(s.name)),
	)
}
