package dcos

// DC/OS DNS records.
const (
	// DNSRecordLeader is a domain name used by the leading Mesos master in a DC/OS cluster.
	DNSRecordLeader = "leader.mesos"

	// DNSRecordMarathonLeader is the domain name used by the leading Marathon master.
	DNSRecordMarathonLeader = "marathon.mesos"

	// DNSRecordMasters is the domain name listing the connected masters in a DC/OS cluster.
	DNSRecordMasters = "master.mesos"
)
