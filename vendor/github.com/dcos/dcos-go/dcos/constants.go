package dcos

import (
	"runtime"
)

// DC/OS roles.
const (
	// RoleMaster defines a master role.
	RoleMaster = "master"

	// RoleAgent defines an agent role.
	RoleAgent = "agent"

	// RoleAgentPublic defines a public agent role.
	RoleAgentPublic = "agent_public"
)

// GetFileDetectIPLocation is a shell script on every DC/OS node which provides IP address used by mesos.
func GetFileDetectIPLocation() string {
	switch runtime.GOOS {
	case "windows":
		return "/opt/mesosphere/bin/detect_ip.ps1"
	default:
		return "/opt/mesosphere/bin/detect_ip"
	}
}
