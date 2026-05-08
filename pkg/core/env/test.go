package env

import (
	"os"
	"strings"
)

func getIsTest() bool {
	return strings.HasSuffix(os.Args[0], ".test")
}
