package router

import (
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
)

var (
	options sync.Map

	allMethods = []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPatch,
		http.MethodPut,
		http.MethodDelete,
	}

	acceptedHeaders = []string{
		"Origin",
		"Content-Type",
		"Content-Length",
		"X-Requested-With",
		"Accept-Encoding",
		"Authorization",
	}
)

// HealthMiddleware is a middleware that responds to health checks
func HealthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// OptionsMiddleware ensures that we return the correct headers for CORS requests
func OptionsMiddleware(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx, _ := r.Context().Value(chi.RouteCtxKey).(*chi.Context)

		var path string
		if r.URL.RawPath != "" {
			path = r.URL.RawPath
		} else {
			path = r.URL.Path
		}

		var methodsStr string
		cached, ok := options.Load(path)
		if ok {
			methodsStr = cached.(string)
		} else {
			var methods []string
			for _, method := range allMethods {
				nctx := chi.NewRouteContext()
				if ctx.Routes.Match(nctx, method, path) {
					methods = append(methods, method)
				}
			}

			methods = append(methods, http.MethodOptions)
			methodsStr = strings.Join(methods, ", ")
			options.Store(path, methodsStr)
		}

		// allowed methods
		w.Header().Set("Allow", methodsStr)

		// allowed methods for CORS
		w.Header().Set("Access-Control-Allow-Methods", methodsStr)

		// allowed origins
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// allowed headers
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(acceptedHeaders, ", "))

		// actually handle the request
		if r.Method != http.MethodOptions {
			h.ServeHTTP(w, r)
			return
		}

		// handle OPTIONS requests
		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}
