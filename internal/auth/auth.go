package auth

import (
	"net/http"
	"strings"
)

type Auth struct {
	apiKey string
}

func New(apiKey string) *Auth {
	return &Auth{apiKey: apiKey}
}

// AuthMiddleware is a middleware that checks for a valid API key
func (a *Auth) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rpc") {
			next.ServeHTTP(w, r)
			return
		}

		apiKey := r.Header.Get("Authorization")
		apiKey = strings.TrimPrefix(apiKey, "Bearer ")
		if apiKey == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if apiKey != a.apiKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
