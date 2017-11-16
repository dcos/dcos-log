package reader

import "fmt"

// Formatter is an interface for formatter functions.
type Formatter func(l Line) string

// SSEFormat implement server sent events format.
func SSEFormat(l Line) (output string) {
	if l.Offset > 0 && l.Size > 0 {
		output += fmt.Sprintf("id: %d\n", l.Offset+l.Size)
	}

	output += fmt.Sprintf("data: %s\n\n", l.Message)
	return output
}

// LineFormat is a simple \n separates format.
func LineFormat(l Line) string {
	return l.Message + "\n"
}
