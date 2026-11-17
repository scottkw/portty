package main

import "strings"

// CLIRemoteSession is a simplified remote session representation for CLI use.
type CLIRemoteSession struct {
	ID       string // session ID (without hostname prefix)
	Name     string
	Status   string
	Hostname string // peer hostname (for display)
	FQDN     string // peer FQDN stripped of trailing dot (for URL construction)
}

// parseRemoteID splits a session identifier on the first colon character.
// If no colon is present, it returns ("", id, false).
// If a colon is found, it returns (hostname, sessionID, true).
func parseRemoteID(id string) (hostname, sessionID string, isRemote bool) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return "", id, false
}
