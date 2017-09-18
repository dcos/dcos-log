package reader

import "fmt"

type Formatter func(l Line) string

func sseFormat(l Line) (output string) {
	if l.Offset > 0 && l.Size > 0 {
		output += fmt.Sprintf("id: %d\n", l.Offset+l.Size)
	}

	output += fmt.Sprintf("data: %s\n\n", l.Message)
	return output
}
