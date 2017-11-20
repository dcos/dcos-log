package reader

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
)

// Formatter is an interface for formatter functions.
type Formatter func(l Line, rm *ReadManager) string

// SSEFormat implement server sent events format.
func SSEFormat(l Line, rm *ReadManager) (output string) {

	var line Line
	// try using the json in response
	jsonLine, err := jsonifyLine(l, rm)
	if err == nil {
		line = *jsonLine
	} else {
		logrus.Errorf("error getting structured message, falling back to simple text")
		line = l
	}

	if line.Offset > 0 && line.Size > 0 {
		output += fmt.Sprintf("id: %d\n", line.Offset+line.Size)
	}

	output += fmt.Sprintf("data: %s\n\n", line.Message)
	return output
}

// LineFormat is a simple \n separates format.
func LineFormat(l Line, rm *ReadManager) string {
	return l.Message + "\n"
}

func jsonifyLine(l Line, rm *ReadManager) (*Line, error) {
	msg := l.Message
	structMsg := struct {
		Fields map[string]interface{} `json:"fields"`
	}{
		Fields: map[string]interface{}{"MESSAGE": msg, "AGENT_ID": rm.agentID, "EXECUTOR_ID": rm.executorID,
			"FRAMEWORK_ID": rm.frameworkID, "CONTAINER_ID": rm.containerID, "FILE": rm.file},
	}

	marshaledStructMessage, err := json.Marshal(structMsg)
	if err != nil {
		return nil, err
	}

	newLine := l
	newLine.Message = string(marshaledStructMessage)

	return &newLine, nil
}
