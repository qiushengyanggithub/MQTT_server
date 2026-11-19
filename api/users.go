// SPDX-License-Identifier: MIT

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/packets"
)

type usersFile struct {
	Users auth.Users `json:"users"`
}

// ACLView is the API representation of an ACL rule.
type ACLView struct {
	Filter     string `json:"filter"`
	Access     byte   `json:"access"`
	AccessName string `json:"access_name"`
}

// UserView is the API representation of an MQTT user.
type UserView struct {
	Username    string    `json:"username"`
	Password    string    `json:"password"`
	Disallow    bool      `json:"disallow"`
	ACL         []ACLView `json:"acl"`
	Online      bool      `json:"online"`
	Connections int       `json:"connections"`
}

// ACLInput is a writable ACL rule.
type ACLInput struct {
	Filter string `json:"filter"`
	Access byte   `json:"access"`
}

// UserInput is a writable MQTT user.
type UserInput struct {
	Username string     `json:"username"`
	Password string     `json:"password"`
	Disallow bool       `json:"disallow"`
	ACL      []ACLInput `json:"acl"`
}

// UserStore manages users inside an auth.Ledger and persists them to disk.
type UserStore struct {
	ledger *auth.Ledger
	path   string
}

// NewUserStore creates a store and loads existing users from path.
// An empty path disables persistence.
func NewUserStore(ledger *auth.Ledger, path string) (*UserStore, error) {
	if ledger == nil {
		return nil, errors.New("auth ledger is nil")
	}
	s := &UserStore{ledger: ledger, path: path}
	if path == "" {
		return s, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return s, nil
	}

	var stored usersFile
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	ledger.Lock()
	if stored.Users != nil {
		ledger.Users = stored.Users
	}
	ledger.Unlock()
	return s, nil
}

func (s *UserStore) persistLocked() error {
	if s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(usersFile{Users: s.ledger.Users}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func accessName(access auth.Access) string {
	switch access {
	case auth.Deny:
		return "拒绝"
	case auth.ReadOnly:
		return "只读"
	case auth.WriteOnly:
		return "只写"
	case auth.ReadWrite:
		return "读写"
	default:
		return "未知"
	}
}

func aclToViews(filters auth.Filters) []ACLView {
	out := make([]ACLView, 0, len(filters))
	for filter, access := range filters {
		out = append(out, ACLView{
			Filter:     string(filter),
			Access:     byte(access),
			AccessName: accessName(access),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Filter < out[j].Filter })
	return out
}

func aclFromInput(list []ACLInput) (auth.Filters, error) {
	filters := auth.Filters{}
	for _, item := range list {
		filter := strings.TrimSpace(item.Filter)
		if filter == "" {
			return nil, errors.New("ACL topic filter 不能为空")
		}
		if item.Access > byte(auth.ReadWrite) {
			return nil, fmt.Errorf("ACL %s 的权限值无效", filter)
		}
		filters[auth.RString(filter)] = auth.Access(item.Access)
	}
	return filters, nil
}

// List returns all users sorted by username.
func (s *UserStore) List() []UserView {
	s.ledger.Lock()
	defer s.ledger.Unlock()

	out := make([]UserView, 0, len(s.ledger.Users))
	for username, rule := range s.ledger.Users {
		out = append(out, UserView{
			Username: username,
			Password: string(rule.Password),
			Disallow: rule.Disallow,
			ACL:      aclToViews(rule.ACL),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out
}

// Create adds a new MQTT user.
func (s *UserStore) Create(in UserInput) (UserView, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return UserView{}, errors.New("用户名不能为空")
	}
	if in.Password == "" {
		return UserView{}, errors.New("新建用户必须设置密码")
	}
	filters, err := aclFromInput(in.ACL)
	if err != nil {
		return UserView{}, err
	}

	s.ledger.Lock()
	defer s.ledger.Unlock()
	if s.ledger.Users == nil {
		s.ledger.Users = auth.Users{}
	}
	if _, exists := s.ledger.Users[username]; exists {
		return UserView{}, fmt.Errorf("用户 %s 已存在", username)
	}

	s.ledger.Users[username] = auth.UserRule{
		Username: auth.RString(username),
		Password: auth.RString(in.Password),
		ACL:      filters,
		Disallow: in.Disallow,
	}
	if err := s.persistLocked(); err != nil {
		delete(s.ledger.Users, username)
		return UserView{}, err
	}

	return UserView{
		Username: username,
		Password: in.Password,
		Disallow: in.Disallow,
		ACL:      aclToViews(filters),
	}, nil
}

// Update modifies an existing user. An empty password keeps the current one.
func (s *UserStore) Update(oldUsername string, in UserInput) (UserView, error) {
	oldUsername = strings.TrimSpace(oldUsername)
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return UserView{}, errors.New("用户名不能为空")
	}

	s.ledger.Lock()
	defer s.ledger.Unlock()

	rule, exists := s.ledger.Users[oldUsername]
	if !exists {
		return UserView{}, fmt.Errorf("用户 %s 不存在", oldUsername)
	}
	if username != oldUsername {
		if _, taken := s.ledger.Users[username]; taken {
			return UserView{}, fmt.Errorf("用户 %s 已存在", username)
		}
	}

	filters, err := aclFromInput(in.ACL)
	if err != nil {
		return UserView{}, err
	}
	if in.Password != "" {
		rule.Password = auth.RString(in.Password)
	}
	rule.Username = auth.RString(username)
	rule.ACL = filters
	rule.Disallow = in.Disallow

	delete(s.ledger.Users, oldUsername)
	s.ledger.Users[username] = rule
	if err := s.persistLocked(); err != nil {
		delete(s.ledger.Users, username)
		s.ledger.Users[oldUsername] = rule
		return UserView{}, err
	}

	return UserView{
		Username: username,
		Password: string(rule.Password),
		Disallow: rule.Disallow,
		ACL:      aclToViews(filters),
	}, nil
}

// SetDisallow enables or disables a user.
func (s *UserStore) SetDisallow(username string, disallow bool) error {
	username = strings.TrimSpace(username)

	s.ledger.Lock()
	defer s.ledger.Unlock()

	rule, exists := s.ledger.Users[username]
	if !exists {
		return fmt.Errorf("用户 %s 不存在", username)
	}
	rule.Disallow = disallow
	s.ledger.Users[username] = rule
	return s.persistLocked()
}

// Delete removes a user.
func (s *UserStore) Delete(username string) error {
	username = strings.TrimSpace(username)

	s.ledger.Lock()
	defer s.ledger.Unlock()

	rule, exists := s.ledger.Users[username]
	if !exists {
		return fmt.Errorf("用户 %s 不存在", username)
	}
	delete(s.ledger.Users, username)
	if err := s.persistLocked(); err != nil {
		s.ledger.Users[username] = rule
		return err
	}
	return nil
}

func (s *Server) requireUserStore(c *gin.Context) bool {
	if s.users == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "用户管理未启用"})
		return false
	}
	return true
}

func (s *Server) listUsers(c *gin.Context) {
	if !s.requireUserStore(c) {
		return
	}

	users := s.users.List()
	online := map[string]int{}
	for _, cl := range s.broker.Clients.GetAll() {
		cl.RLock()
		username := string(cl.Properties.Username)
		cl.RUnlock()
		if username != "" && !cl.Closed() {
			online[username]++
		}
	}
	for i := range users {
		users[i].Connections = online[users[i].Username]
		users[i].Online = users[i].Connections > 0
	}

	c.JSON(http.StatusOK, users)
}

func (s *Server) createUser(c *gin.Context) {
	if !s.requireUserStore(c) {
		return
	}
	var in UserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := s.users.Create(in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (s *Server) updateUser(c *gin.Context) {
	if !s.requireUserStore(c) {
		return
	}
	var in UserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := s.users.Update(c.Param("username"), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (s *Server) setUserDisallowed(disallow bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.requireUserStore(c) {
			return
		}
		username := c.Param("username")
		if err := s.users.SetDisallow(username, disallow); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		kicked := 0
		if disallow {
			for _, cl := range s.broker.Clients.GetAll() {
				if cl.Closed() {
					continue
				}
				cl.RLock()
				connectedUser := string(cl.Properties.Username)
				cl.RUnlock()
				if connectedUser == username {
					_ = s.broker.DisconnectClient(cl, packets.ErrAdministrativeAction)
					kicked++
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "kicked": kicked})
	}
}

func (s *Server) deleteUser(c *gin.Context) {
	if !s.requireUserStore(c) {
		return
	}
	if err := s.users.Delete(c.Param("username")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
