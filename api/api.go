// SPDX-License-Identifier: MIT

// Package api provides a Gin based HTTP layer for the MQTT broker and serves
// the built Vue3 dashboard from the same process.
package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// Config configures the web API server.
type Config struct {
	Addr        string
	Dist        string
	Token       string
	WebUser     string
	WebPassword string
	UserStore   *UserStore
}

// Server owns the Gin engine and its MQTT bridge hook.
type Server struct {
	broker      *mqtt.Server
	hook        *BrokerHook
	engine      *gin.Engine
	httpServer  *http.Server
	addr        string
	dist        string
	token       string
	webUser     string
	webPassword string
	sessionsMu  sync.Mutex
	sessions    map[string]time.Time
	users       *UserStore
}

// New attaches a publish hook to the broker and builds the Gin routes.
func New(broker *mqtt.Server, cfg Config) (*Server, error) {
	hook := NewBrokerHook(256)
	if err := broker.AddHook(hook, nil); err != nil {
		return nil, err
	}

	s := &Server{
		broker:      broker,
		hook:        hook,
		addr:        cfg.Addr,
		dist:        cfg.Dist,
		token:       cfg.Token,
		webUser:     cfg.WebUser,
		webPassword: cfg.WebPassword,
		sessions:    make(map[string]time.Time),
		users:       cfg.UserStore,
	}
	if s.addr == "" {
		s.addr = ":8081"
	}
	if s.dist == "" {
		s.dist = "web/dist"
	}

	s.engine = gin.New()
	s.engine.Use(gin.Logger(), gin.Recovery())
	s.engine.Use(corsMiddleware())
	s.routes()

	return s, nil
}

// Engine returns the configured Gin engine, useful for tests or custom hosts.
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// Serve starts the HTTP server. ErrServerClosed is treated as a normal stop.
func (s *Server) Serve() error {
	s.httpServer = &http.Server{
		Addr:              s.addr,
		Handler:           s.engine,
		ReadHeaderTimeout: 5 * time.Second,
	}
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Close gracefully stops the HTTP server.
func (s *Server) Close(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) routes() {
	public := s.engine.Group("/api")
	public.POST("/login", s.login)
	public.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := s.engine.Group("/api", s.authMiddleware())
	{
		api.POST("/logout", s.logout)
		api.GET("/overview", s.overview)
		api.GET("/clients", s.clients)
		api.DELETE("/clients/:id", s.kickClient)
		api.GET("/subscriptions", s.subscriptions)
		api.GET("/retained", s.retained)
		api.GET("/messages/stream", s.streamMessages)
		api.POST("/publish", s.publish)
		api.GET("/users", s.listUsers)
		api.POST("/users", s.createUser)
		api.PUT("/users/:username", s.updateUser)
		api.DELETE("/users/:username", s.deleteUser)
		api.POST("/users/:username/disable", s.setUserDisallowed(true))
		api.POST("/users/:username/enable", s.setUserDisallowed(false))
	}

	s.static()
}

func requestToken(c *gin.Context) string {
	if token := c.Query("token"); token != "" {
		return token
	}
	header := c.GetHeader("Authorization")
	if len(header) > 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

func (s *Server) validSession(token string) bool {
	if token == "" {
		return false
	}
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	expires, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expires) {
		delete(s.sessions, token)
		return false
	}
	return true
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.token == "" && s.webUser == "" {
			c.Next()
			return
		}

		token := requestToken(c)
		if (s.token == "" || token != s.token) && !s.validSession(token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (s *Server) login(c *gin.Context) {
	if s.webUser == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "登录未启用"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Username != s.webUser || req.Password != s.webPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	token := hex.EncodeToString(buf)
	s.sessionsMu.Lock()
	s.sessions[token] = time.Now().Add(8 * time.Hour)
	s.sessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"token": token, "expires_in": 28800})
}

func (s *Server) logout(c *gin.Context) {
	token := requestToken(c)
	s.sessionsMu.Lock()
	delete(s.sessions, token)
	s.sessionsMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *Server) static() {
	info, err := os.Stat(s.dist)
	if err != nil || !info.IsDir() {
		s.engine.NoRoute(func(c *gin.Context) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(
				"<!doctype html><html><body style=\"font-family:sans-serif;padding:40px\">"+
					"<h1>Mochi MQTT Web</h1><p>Frontend not built.</p>"+
					"<p>Run <code>npm install &amp;&amp; npm run build</code> in <code>web/</code>.</p></body></html>"))
		})
		return
	}

	s.engine.Static("/assets", filepath.Join(s.dist, "assets"))
	s.engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.File(filepath.Join(s.dist, "index.html"))
	})
}

func (s *Server) overview(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"info":      s.broker.Info.Clone(),
		"clients":   len(s.broker.Clients.GetAll()),
		"listeners": s.broker.Listeners.Len(),
	})
}

type clientView struct {
	ID              string             `json:"id"`
	Remote          string             `json:"remote"`
	Listener        string             `json:"listener"`
	Username        string             `json:"username"`
	ProtocolVersion byte               `json:"protocol_version"`
	Clean           bool               `json:"clean"`
	Keepalive       uint16             `json:"keepalive"`
	Connected       bool               `json:"connected"`
	Inflight        int                `json:"inflight"`
	Subscriptions   []subscriptionView `json:"subscriptions"`
}

type subscriptionView struct {
	ClientID string `json:"client_id"`
	Filter   string `json:"filter"`
	Qos      byte   `json:"qos"`
	Shared   string `json:"shared,omitempty"`
}

func (s *Server) clients(c *gin.Context) {
	all := s.broker.Clients.GetAll()
	out := make([]clientView, 0, len(all))
	for _, cl := range all {
		cl.RLock()
		username := string(cl.Properties.Username)
		protocol := cl.Properties.ProtocolVersion
		clean := cl.Properties.Clean
		keepalive := cl.State.Keepalive
		subs := cl.State.Subscriptions.GetAll()
		cl.RUnlock()

		subViews := make([]subscriptionView, 0, len(subs))
		for _, sub := range subs {
			subViews = append(subViews, subscriptionView{
				ClientID: cl.ID,
				Filter:   sub.Filter,
				Qos:      sub.Qos,
				Shared:   strings.Join(sub.ShareName, "/"),
			})
		}
		sort.Slice(subViews, func(i, j int) bool { return subViews[i].Filter < subViews[j].Filter })

		out = append(out, clientView{
			ID:              cl.ID,
			Remote:          cl.Net.Remote,
			Listener:        cl.Net.Listener,
			Username:        username,
			ProtocolVersion: protocol,
			Clean:           clean,
			Keepalive:       keepalive,
			Connected:       !cl.Closed(),
			Inflight:        cl.State.Inflight.Len(),
			Subscriptions:   subViews,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	c.JSON(http.StatusOK, out)
}

func (s *Server) kickClient(c *gin.Context) {
	cl, ok := s.broker.Clients.Get(c.Param("id"))
	if !ok || cl.Closed() {
		c.JSON(http.StatusNotFound, gin.H{"error": "客户端不存在或已断开"})
		return
	}
	if err := s.broker.DisconnectClient(cl, packets.ErrAdministrativeAction); err != nil && err != packets.ErrAdministrativeAction {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) subscriptions(c *gin.Context) {
	clients := s.broker.Clients.GetAll()
	var out []subscriptionView
	for _, cl := range clients {
		cl.RLock()
		subs := cl.State.Subscriptions.GetAll()
		cl.RUnlock()
		for _, sub := range subs {
			out = append(out, subscriptionView{
				ClientID: cl.ID,
				Filter:   sub.Filter,
				Qos:      sub.Qos,
				Shared:   strings.Join(sub.ShareName, "/"),
			})
		}
	}
	if out == nil {
		out = []subscriptionView{}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Filter == out[j].Filter {
			return out[i].ClientID < out[j].ClientID
		}
		return out[i].Filter < out[j].Filter
	})
	c.JSON(http.StatusOK, out)
}

type retainedView struct {
	Topic     string `json:"topic"`
	Payload   string `json:"payload,omitempty"`
	Base64    string `json:"payload_base64,omitempty"`
	Qos       byte   `json:"qos"`
	Created   int64  `json:"created"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

func (s *Server) retained(c *gin.Context) {
	all := s.broker.Topics.Retained.GetAll()
	out := make([]retainedView, 0, len(all))
	for topic, pk := range all {
		view := retainedView{
			Topic:     topic,
			Qos:       pk.FixedHeader.Qos,
			Created:   pk.Created,
			ExpiresAt: pk.Expiry,
		}
		if utf8.Valid(pk.Payload) {
			view.Payload = string(pk.Payload)
		} else {
			view.Base64 = base64.StdEncoding.EncodeToString(pk.Payload)
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Topic < out[j].Topic })
	c.JSON(http.StatusOK, out)
}

type publishRequest struct {
	Topic   string `json:"topic" binding:"required"`
	Payload string `json:"payload"`
	Qos     byte   `json:"qos"`
	Retain  bool   `json:"retain"`
}

func (s *Server) publish(c *gin.Context) {
	var req publishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Qos > 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "qos must be 0, 1 or 2"})
		return
	}
	if err := s.broker.Publish(req.Topic, []byte(req.Payload), req.Retain, req.Qos); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": err.Error(),
			"hint":  "start the broker with inline_client enabled",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) streamMessages(c *gin.Context) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case msg := <-s.hook.Messages():
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(c.Writer, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
		}
	}
}
