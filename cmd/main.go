// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 mochi-mqtt, mochi-co
// SPDX-FileContributor: mochi-co

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/api"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
	tcpAddr := flag.String("tcp", ":1883", "network address for TCP listener")
	wsAddr := flag.String("ws", ":1882", "network address for Websocket listener")
	infoAddr := flag.String("info", ":8080", "network address for web info dashboard listener")
	tlsCertFile := flag.String("tls-cert-file", "", "TLS certificate file")
	tlsKeyFile := flag.String("tls-key-file", "", "TLS key file")
	webAddr := flag.String("web-addr", ":8081", "network address for the web dashboard and REST API (empty to disable)")
	webDist := flag.String("web-dist", "web/dist", "path to the built Vue3 frontend")
	webToken := flag.String("web-token", "", "optional static bearer token for the web API")
	webUser := flag.String("web-user", "admin", "username for the web dashboard login")
	webPassword := flag.String("web-password", "@admin@123", "password for the web dashboard login")
	usersFile := flag.String("users-file", "users.json", "path to the MQTT user database (empty to disable persistence)")
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	var tlsConfig *tls.Config

	if tlsCertFile != nil && tlsKeyFile != nil && *tlsCertFile != "" && *tlsKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(*tlsCertFile, *tlsKeyFile)
		if err != nil {
			return
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	server := mqtt.New(&mqtt.Options{InlineClient: true})

	ledger := &auth.Ledger{}
	userStore, err := api.NewUserStore(ledger, *usersFile)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.AddHook(&api.UserAuthHook{Store: userStore}, nil); err != nil {
		log.Fatal(err)
	}

	tcp := listeners.NewTCP(listeners.Config{
		ID:        "t1",
		Address:   *tcpAddr,
		TLSConfig: tlsConfig,
	})
	err = server.AddListener(tcp)
	if err != nil {
		log.Fatal(err)
	}

	ws := listeners.NewWebsocket(listeners.Config{
		ID:      "ws1",
		Address: *wsAddr,
	})
	err = server.AddListener(ws)
	if err != nil {
		log.Fatal(err)
	}

	stats := listeners.NewHTTPStats(
		listeners.Config{
			ID:      "info",
			Address: *infoAddr,
		},
		server.Info,
	)
	err = server.AddListener(stats)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	var web *api.Server
	if *webAddr != "" {
		web, err = api.New(server, api.Config{
			Addr:        *webAddr,
			Dist:        *webDist,
			Token:       *webToken,
			WebUser:     *webUser,
			WebPassword: *webPassword,
			UserStore:   userStore,
		})
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			log.Printf("web dashboard listening on %s", *webAddr)
			if err := web.Serve(); err != nil {
				log.Fatal(err)
			}
		}()
	}

	<-done
	server.Log.Warn("caught signal, stopping...")
	if web != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = web.Close(ctx)
		cancel()
	}
	_ = server.Close()
	server.Log.Info("mochi mqtt shutdown complete")
}
