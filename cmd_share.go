package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/scottkw/portty/internal/daemon"
)

// resolveSession finds a session by exact name, then exact ID, then a unique
// ID prefix. Errors carry the CLI copy, including the `portty: ` prefix.
func resolveSession(sessions []daemon.SessionInfo, arg string) (daemon.SessionInfo, error) {
	if arg == "" {
		// An unset shell variable must not match a session with an empty name.
		return daemon.SessionInfo{}, noSessionError{arg}
	}
	var named []daemon.SessionInfo
	for _, s := range sessions {
		if s.Name == arg {
			named = append(named, s)
		}
	}
	if len(named) == 1 {
		return named[0], nil
	}
	if len(named) > 1 {
		prefixes := make([]string, len(named))
		for i, s := range named {
			prefixes[i] = s.ID
			if len(s.ID) > 8 {
				prefixes[i] = s.ID[:8]
			}
		}
		return daemon.SessionInfo{}, fmt.Errorf("portty: more than one session is named %q. Use an ID prefix instead: %s.", arg, strings.Join(prefixes, ", "))
	}
	var byPrefix []daemon.SessionInfo
	for _, s := range sessions {
		if s.ID == arg {
			return s, nil
		}
		if arg != "" && strings.HasPrefix(s.ID, arg) {
			byPrefix = append(byPrefix, s)
		}
	}
	if len(byPrefix) == 1 {
		return byPrefix[0], nil
	}
	return daemon.SessionInfo{}, noSessionError{arg}
}

// noSessionError is resolveSession's no-match error; attach falls back to a
// host lookup on it.
type noSessionError struct{ arg string }

func (e noSessionError) Error() string {
	return fmt.Sprintf("portty: no session named %q. Run 'portty list' to see your sessions.", e.arg)
}

func lookupSession(client *daemon.DaemonClient, arg string) (daemon.SessionInfo, error) {
	sessions, err := client.ListSessions()
	if err != nil {
		return daemon.SessionInfo{}, fmt.Errorf("portty: %w", err)
	}
	return resolveSession(sessions, arg)
}

// minDestructivePrefix is the shortest ID prefix kill, rename and unshare accept. A
// short name abbreviation like "b" must not fall through to a hex ID prefix and
// hit an unrelated session; 8 matches the prefixes the ambiguity error prints.
const minDestructivePrefix = 8

// lookupSessionStrict is lookupSession for destructive commands: a match by ID
// prefix needs at least minDestructivePrefix characters.
func lookupSessionStrict(client *daemon.DaemonClient, arg string) (daemon.SessionInfo, error) {
	s, err := lookupSession(client, arg)
	if err != nil {
		return s, err
	}
	if s.Name != arg && s.ID != arg && len(arg) < minDestructivePrefix {
		return daemon.SessionInfo{}, noSessionError{arg}
	}
	return s, nil
}

// cmdShare turns sharing on and prints the full-control link.
//
//	portty share <session>
func cmdShare(client *daemon.DaemonClient, args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: portty share <session>")
	}
	s, err := lookupSession(client, args[0])
	if err != nil {
		return err
	}
	resp, err := client.SetSessionShared(s.ID, true)
	if err != nil {
		var se *daemon.ShareError
		switch {
		case errors.As(err, &se) && se.Code == daemon.ShareErrTailscaleOff:
			return errors.New("portty: Tailscale isn't connected. Remote access and sharing are off until it is.")
		case errors.As(err, &se) && se.Code == daemon.ShareErrHTTPSOff:
			return errors.New("portty: couldn't start sharing: HTTPS certificates are off for your tailnet. Turn on HTTPS on the DNS page of the Tailscale admin console, then try again.")
		case errors.As(err, &se):
			return fmt.Errorf("portty: couldn't start sharing: %s. Check that Tailscale is connected, then try again.", se.Detail)
		default:
			return fmt.Errorf("portty: couldn't start sharing: %v. Check that Tailscale is connected, then try again.", err)
		}
	}
	fmt.Fprintf(out, "Shared %q.\n%s\nTreat this link like a password. It only opens on devices signed in to your Tailscale account.\n", s.Name, resp.URL)
	if resp.FirstShare {
		host := resp.URL
		if u, err := url.Parse(resp.URL); err == nil {
			host = u.Hostname()
		}
		fmt.Fprintf(out, "The first share gets an HTTPS certificate for %s. That name is published in public Certificate Transparency logs.\n", host)
	}
	return nil
}

// cmdUnshare turns sharing off, which revokes the link and disconnects viewers.
// The argument resolves like kill (exact name, exact ID, unique ID prefix of 8+
// characters), so an unknown or short argument unshares nothing.
//
//	portty unshare <session>
func cmdUnshare(client *daemon.DaemonClient, args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: portty unshare <session>")
	}
	s, err := lookupSessionStrict(client, args[0])
	if err != nil {
		return err
	}
	if !s.WebEnabled {
		fmt.Fprintf(out, "%q isn't shared.\n", s.Name)
		return nil
	}
	if _, err := client.SetSessionShared(s.ID, false); err != nil {
		detail := err.Error()
		var se *daemon.ShareError
		if errors.As(err, &se) {
			detail = se.Detail
		}
		return fmt.Errorf("portty: couldn't stop sharing: %s. Try again.", detail)
	}
	fmt.Fprintf(out, "Stopped sharing %q. Remote viewers were disconnected.\n", s.Name)
	return nil
}
