package color

import "fmt"

const (
	Reset  = "\033[0m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
)

func BlueString(s string) string {
	return fmt.Sprintf("%s%s%s", Blue, s, Reset)
}

func YellowString(s string) string {
	return fmt.Sprintf("%s%s%s", Yellow, s, Reset)
}

func GreenString(s string) string {
	return fmt.Sprintf("%s%s%s", Green, s, Reset)
}
