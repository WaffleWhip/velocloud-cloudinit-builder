package output

import "fmt"

var quiet bool

// SetQuiet toggles whether console output should be suppressed.
func SetQuiet(v bool) {
	quiet = v
}

// Println prints a line unless quiet mode is enabled.
func Println(msg string) {
	if quiet {
		return
	}
	fmt.Println(msg)
}

// Printf prints formatted text unless quiet mode is enabled.
func Printf(format string, args ...interface{}) {
	if quiet {
		return
	}
	fmt.Printf(format, args...)
}
