package file

import (
	"encoding/hex"
	"fmt"
	"os"
)

func (s *Domain) getFileName(name string) string {
	return fmt.Sprintf(
		"%s%csession_%s.dat",
		s.dir,
		os.PathSeparator,
		hex.EncodeToString([]byte(name)),
	)
}
