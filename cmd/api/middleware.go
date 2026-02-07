package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"

	"fileforge/internal/models"
)

type contextKey string

const sessionCtxKey contextKey = "session"

func sessionFromCtx(r *http.Request) *models.Session {
	s, _ := r.Context().Value(sessionCtxKey).(*models.Session)
	return s
}

func (a *app) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if ip == "" {
			writeError(w, http.StatusBadRequest, "Could not determine client IP")
			return
		}

		session, err := a.db.TouchSession(r.Context(), ip, a.cfg.FlagThreshold)
		if err != nil {
			log.Printf("[session] touch error for %s: %v", ip, err)
			writeError(w, http.StatusInternalServerError, "Session error")
			return
		}

		if session.IsFlagged {
			log.Printf("[session] Blocked flagged IP: %s (total: %d)",
				ip, session.TotalRequestCount)
			writeError(w, http.StatusForbidden,
				"Access restricted. Too many requests from this IP.")
			return
		}

		if session.HourlyRequestCount > a.cfg.RateLimitPerHour {
			log.Printf("[session] Rate limited: %s (%d/%d this hour)",
				ip, session.HourlyRequestCount, a.cfg.RateLimitPerHour)

			w.Header().Set("Retry-After", "3600")
			writeError(w, http.StatusTooManyRequests,
				"Rate limit exceeded. Please try again later.")
			return
		}

		ctx := context.WithValue(r.Context(), sessionCtxKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}


func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Printf("[json] encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, models.ErrorResponse{Error: msg})
}