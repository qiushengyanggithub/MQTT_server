// SPDX-License-Identifier: MIT

package api

import (
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/packets"
)

// UserAuthHook enforces the users managed through the web dashboard.
// When no users exist it preserves the original allow-all behaviour.
type UserAuthHook struct {
	mqtt.HookBase
	Store *UserStore
}

func (h *UserAuthHook) ID() string {
	return "web-user-auth"
}

func (h *UserAuthHook) Provides(b byte) bool {
	return b == mqtt.OnConnectAuthenticate || b == mqtt.OnACLCheck
}

func (h *UserAuthHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	return h.Store.Authenticate(cl, pk)
}

func (h *UserAuthHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return h.Store.ACLAllowed(cl, topic, write)
}

// Authenticate checks username/password against the managed user list.
func (s *UserStore) Authenticate(cl *mqtt.Client, pk packets.Packet) bool {
	s.ledger.Lock()
	defer s.ledger.Unlock()

	if len(s.ledger.Users) == 0 {
		return true
	}
	rule, ok := s.ledger.Users[string(cl.Properties.Username)]
	if !ok || rule.Disallow || rule.Password == "" {
		return false
	}
	return rule.Password == auth.RString(pk.Connect.Password)
}

// ACLAllowed checks topic permissions for a managed user.
func (s *UserStore) ACLAllowed(cl *mqtt.Client, topic string, write bool) bool {
	s.ledger.Lock()
	defer s.ledger.Unlock()

	if len(s.ledger.Users) == 0 {
		return true
	}
	rule, ok := s.ledger.Users[string(cl.Properties.Username)]
	if !ok || rule.Disallow {
		return false
	}
	if len(rule.ACL) == 0 {
		return true
	}

	for filter, access := range rule.ACL {
		if _, matched := auth.MatchTopic(string(filter), topic); !matched {
			continue
		}
		if !write && (access == auth.ReadOnly || access == auth.ReadWrite) {
			return true
		}
		if write && (access == auth.WriteOnly || access == auth.ReadWrite) {
			return true
		}
		return false
	}
	return false
}
