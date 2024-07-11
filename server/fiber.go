package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func StartFiberServer(cfg *Config) error {
	app := fiber.New()

	app.Static("/", "./public")

	app.Post("/login", func(c *fiber.Ctx) error {
		credentials := &Credentials{}
		if err := c.BodyParser(&credentials); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid credentials payload")
		}
		if credentials.Email != "demo" || credentials.Password != "demo" {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid email/password")
		}

		if c.Cookies(cfg.SessionKey) != "" {
			return c.SendString("already authenticated")
		}

		userId := 1 // validate credentials and find user

		sessionId := uuid.NewString()
		sessionExpiration := time.Now().Add(1 * time.Hour)
		refreshId := uuid.NewString()
		refreshExpiration := time.Now().Add(24 * time.Hour)

		_, err := cfg.KvClient.TxPipelined(c.UserContext(), func(pl redis.Pipeliner) error {
			if err := pl.Set(c.UserContext(), sessionId, userId, time.Until(sessionExpiration)).Err(); err != nil {
				return err
			}
			if err := pl.Set(c.UserContext(), refreshId, userId, time.Until(refreshExpiration)).Err(); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			cfg.Logger.Println("error storing session keys", err)
			return fiber.NewError(fiber.StatusInternalServerError, "failed to authenticate")
		}

		sessionCookie := createFiberSecureCookie(cfg.SessionKey, sessionId, sessionExpiration)
		refreshCookie := createFiberSecureCookie(cfg.RefreshKey, refreshId, refreshExpiration)

		c.Cookie(sessionCookie)
		c.Cookie(refreshCookie)

		return c.SendString("auth success")
	})

	app.Post("/refresh", func(c *fiber.Ctx) error {
		refreshCookie := c.Cookies(cfg.RefreshKey)
		if refreshCookie == "" {
			cfg.Logger.Println("refresh cookie not present")
			return fiber.NewError(fiber.StatusUnauthorized, "failed to refresh session")
		}

		userId, err := cfg.KvClient.Get(c.UserContext(), refreshCookie).Int()
		if err != nil {
			cfg.Logger.Println("error checking refresh key", err)
			return fiber.NewError(fiber.StatusUnauthorized, "failed to refresh session")
		}

		sessionId := uuid.NewString()
		sessionExp := time.Now().Add(1 * time.Hour)

		if err = cfg.KvClient.Set(c.UserContext(), sessionId, userId, time.Until(sessionExp)).Err(); err != nil {
			cfg.Logger.Println("error storing session key")
			return fiber.NewError(http.StatusInternalServerError, "failed to refresh session")
		}

		sessionCookie := createFiberSecureCookie(cfg.SessionKey, sessionId, sessionExp)
		c.Cookie(sessionCookie)

		return c.SendString("session refreshed")
	})

	var userKey UserKey
	app.Use(func(c *fiber.Ctx) error {
		sessionCookie := c.Cookies(cfg.SessionKey)
		if sessionCookie == "" {
			cfg.Logger.Println("session cookie not present")
			return fiber.ErrUnauthorized
		}

		userId, err := cfg.KvClient.Get(c.UserContext(), sessionCookie).Int()
		if err != nil {
			cfg.Logger.Println("session not found", err)
			return fiber.ErrUnauthorized
		}

		c.Locals(userKey, userId)
		return c.Next()
	})

	app.Get("/protected", func(c *fiber.Ctx) error {
		userId := c.Locals(userKey).(int)
		return c.SendString(fmt.Sprintf(`userId from session: %d`, userId))
	})

	app.Get("/clear-cookies", func(c *fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{Name: cfg.SessionKey, Expires: time.Now()})
		c.Cookie(&fiber.Cookie{Name: cfg.RefreshKey, Expires: time.Now()})
		return c.SendString("all cookies cleared")
	})

	app.Get("/clear-session", func(c *fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{Name: cfg.SessionKey, Expires: time.Now()})
		return c.SendString("session cookie cleared")
	})

	app.Get("/clear-refresh", func(c *fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{Name: cfg.RefreshKey, Expires: time.Now()})
		return c.SendString("refresh cookie cleared")
	})

	return app.Listen(cfg.ServerAddr)
}

func createFiberSecureCookie(name string, value string, expiration time.Time) *fiber.Cookie {
	return &fiber.Cookie{
		Name:     name,
		Value:    value,
		Expires:  expiration,
		HTTPOnly: true,
		Secure:   true,
		SameSite: fiber.CookieSameSiteStrictMode,
	}
}
