# dcos-go/elector

A ZK-based leadership election library

## Overview

This library allows multiple nodes to elect a leader. This works by creating ephemeral and sequential znodes as the children of a base node.  The child of the base node that has the lowest sequence number is agreed upon to be the leader.

Note that this elector assumes that on any error from ZK (partition or otherwise) that the state of the election cannot be reliably determined.  If an error is received on the events channel, the client should shut down (and presumably, let the init system restart it).

## Usage

	ident := "127.0.0.1" // set this to your IP address
	basePath := "/services/my-service/leader"
	connector := NewConnection([]string{"127.0.0.1:2181"}, ConnectionOpts{})
	var acl []zk.ACL // set this to something useful, or leave nil

	el, err := Start(ident, basePath, acl, connector)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for event := range el.Events() {
			if event.Err != nil {
				log.Fatal("Leadership failed. Exiting...", event.Err)
			}
			if event.Leader {
				log.Info("I am now the leader")
			} else {
				log.Info("I am not the leader anymore")
			}
		}
	}()

	http.Handle("/v1/leader", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leader := el.LeaderIdent()
		if leader == "" {
			fmt.Fprintln(w, "leader unknown")
			return
		}
		fmt.Fprintf(w, "leader is currently: %s\n", leader)
	}))
