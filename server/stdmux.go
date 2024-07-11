package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func StartStdMuxServer(cfg *Config) error {
	mux := &http.ServeMux{}

	mux.Handle("GET /", http.FileServer(http.Dir("./public")))

	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		credentials := &Credentials{}
		if err := json.NewDecoder(r.Body).Decode(credentials); err != nil {
			http.Error(w, "invalid credentials payload", http.StatusBadRequest)
			return
		}
		if credentials.Email != "demo" || credentials.Password != "demo" {
			http.Error(w, "invalid email/password", http.StatusUnauthorized)
			return
		}

		if _, err := r.Cookie(cfg.SessionKey); err == nil {
			io.WriteString(w, "already authenticated")
			return
		}

		userId := 1 // validate credentials and find user

		sessionId := uuid.NewString()
		sessionExpiration := time.Now().Add(1 * time.Hour)
		refreshId := uuid.NewString()
		refreshExpiration := time.Now().Add(24 * time.Hour)

		_, err := cfg.KvClient.TxPipelined(r.Context(), func(pl redis.Pipeliner) error {
			if err := pl.Set(r.Context(), sessionId, userId, time.Until(sessionExpiration)).Err(); err != nil {
				return err
			}
			if err := pl.Set(r.Context(), refreshId, userId, time.Until(refreshExpiration)).Err(); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			cfg.Logger.Println("error storing session keys", err)
			http.Error(w, "failed to authenticate", http.StatusInternalServerError)
			return
		}

		sessionCookie := createSecureCookie(cfg.SessionKey, sessionId, sessionExpiration)
		refreshCookie := createSecureCookie(cfg.RefreshKey, refreshId, refreshExpiration)

		http.SetCookie(w, sessionCookie)
		http.SetCookie(w, refreshCookie)

		io.WriteString(w, "auth success")
	})

	mux.HandleFunc("POST /refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCookie, err := r.Cookie(cfg.RefreshKey)
		if err != nil {
			cfg.Logger.Println("refresh cookie not present", err)
			http.Error(w, "failed to refresh session", http.StatusUnauthorized)
			return
		}

		userId, err := cfg.KvClient.Get(r.Context(), refreshCookie.Value).Int()
		if err != nil {
			cfg.Logger.Println("error checking refresh key", err)
			http.Error(w, "failed to refresh session", http.StatusUnauthorized)
		}

		sessionId := uuid.NewString()
		sessionExp := time.Now().Add(1 * time.Hour)

		if err = cfg.KvClient.Set(r.Context(), sessionId, userId, time.Until(sessionExp)).Err(); err != nil {
			cfg.Logger.Println("error storing session key")
			http.Error(w, "failed to refresh session", http.StatusInternalServerError)
			return
		}

		sessionCookie := createSecureCookie(cfg.SessionKey, sessionId, sessionExp)
		http.SetCookie(w, sessionCookie)

		io.WriteString(w, "session refreshed")
	})

	var userKey UserKey
	authMiddleware := func(handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionCookie, err := r.Cookie(cfg.SessionKey)
			if err != nil {
				cfg.Logger.Println("session cookie not present", err)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userId, err := cfg.KvClient.Get(r.Context(), sessionCookie.Value).Int()
			if err != nil {
				cfg.Logger.Println("session not found", err)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			handler(w, r.WithContext(context.WithValue(r.Context(), userKey, userId)))
		})
	}

	mux.HandleFunc("GET /protected", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		userId := r.Context().Value(userKey).(int)
		io.WriteString(w, fmt.Sprintf(`userId from session: %d`, userId))
	}))

	mux.HandleFunc("GET /clear-cookies", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: cfg.SessionKey, Expires: time.Now()})
		http.SetCookie(w, &http.Cookie{Name: cfg.RefreshKey, Expires: time.Now()})
		io.WriteString(w, "all cookies cleared")
	})

	mux.HandleFunc("GET /clear-session", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: cfg.SessionKey, Expires: time.Now()})
		io.WriteString(w, "session cookie cleared")
	})

	mux.HandleFunc("GET /clear-refresh", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: cfg.RefreshKey, Expires: time.Now()})
		io.WriteString(w, "refresh cookie cleared")
	})

	return http.ListenAndServe(cfg.ServerAddr, mux)
}

func createSecureCookie(name string, value string, expiration time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Expires:  expiration,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
}
