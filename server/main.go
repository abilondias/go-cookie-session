package main

import (
	"context"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserKey int

type Config struct {
	KvAddr     string
	ServerAddr string
	SessionKey string
	RefreshKey string

	Logger   *log.Logger
	KvClient *redis.Client
}

func main() {
	const kvAddr = "localhost:6666"
	logger := log.New(os.Stdout, "go-cookie-session:", log.LstdFlags)
	ctx := context.Background()
	kvClient := redis.NewClient(&redis.Options{Addr: kvAddr, DB: 0})

	if err := kvClient.Ping(ctx).Err(); err != nil {
		logger.Fatal(err)
	}
	defer func() {
		if err := kvClient.Close(); err != nil {
			logger.Fatal(err)
		}
	}()

	cfg := &Config{
		ServerAddr: "localhost:8888",
		SessionKey: "session",
		RefreshKey: "refresh",

		Logger:   logger,
		KvClient: kvClient,
	}

	logger.Printf("starting server at %s", cfg.ServerAddr)

	err := StartStdMuxServer(cfg)
	// err := StartFiberServer(cfg)
	if err != nil {
		logger.Fatal("failed to start server", err)
	}
}
