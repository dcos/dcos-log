package reader


// Line is a structure for a line message with offset.
type Line struct {
	Message string
	Offset  int
	Size    int
}
