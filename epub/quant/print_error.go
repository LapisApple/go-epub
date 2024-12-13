package quant

import "fmt"

const startRed = "\033[0;31m"

const endColor = "\033[0m"

func PrintError(format string, a ...any) (n int, err error) {
	format = startRed + format + endColor
	return fmt.Printf(format, a...)
}
