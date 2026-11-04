package app

import (
	"compress/gzip"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

//go:embed specifications/*.json
var specifications embed.FS

func operationsAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		password := env("OPS_PASSWORD", "")
		if password == "" {
			write(w, 503, map[string]any{"error": "operations access is not configured", "data_as_of": nil})
			return
		}
		user, pass, ok := r.BasicAuth()
		expectedUser := sha256.Sum256([]byte(env("OPS_USERNAME", "ops")))
		expectedPass := sha256.Sum256([]byte(password))
		actualUser := sha256.Sum256([]byte(user))
		actualPass := sha256.Sum256([]byte(pass))
		if !ok || subtle.ConstantTimeCompare(actualUser[:], expectedUser[:]) != 1 || subtle.ConstantTimeCompare(actualPass[:], expectedPass[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Subscope operations", charset="UTF-8"`)
			write(w, 401, map[string]any{"error": "operations authentication required", "data_as_of": nil})
			return
		}
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/db/static/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func toolProxy(raw, prefix string, strip, grafana bool) (http.Handler, error) {
	target, err := url.Parse(raw)
	if err != nil || target.Scheme != "http" || target.Host == "" || target.User != nil {
		return nil, fmt.Errorf("invalid tools upstream")
	}
	proxy := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) {
		if strip {
			pr.Out.URL.Path = strings.TrimPrefix(pr.Out.URL.Path, strings.TrimSuffix(prefix, "/"))
			pr.Out.URL.RawPath = ""
		}
		pr.SetURL(target)
		pr.SetXForwarded()
		if prefix == "/db/" {
			pr.Out.Host = pr.In.Host
			pr.Out.Header.Set("X-Script-Name", "/db")
			scheme := "http"
			if pr.In.TLS != nil || pr.In.Header.Get("X-Forwarded-Proto") == "https" {
				scheme = "https"
			}
			pr.Out.Header.Set("X-Scheme", scheme)
			pr.Out.Header.Set("X-Forwarded-Proto", scheme)
		}
		pr.Out.Header.Del("Authorization")
		pr.Out.Header.Del("X-WEBAUTH-USER")
		if grafana {
			pr.Out.Header.Set("X-WEBAUTH-USER", env("OPS_USERNAME", "ops"))
		}
	}, ErrorHandler: func(w http.ResponseWriter, r *http.Request, _ error) {
		write(w, 503, map[string]any{"error": "operations service unavailable; enable the observability Compose profile", "data_as_of": nil})
	}}
	proxy.Transport = &http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: 10 * time.Second, IdleConnTimeout: 60 * time.Second}
	if prefix == "/db/" {
		proxy.ModifyResponse = compressPGAdminAsset
	}
	return proxy, nil
}

// pgAdmin serves large JS bundles without compression. Compress only its static
// assets, never SQL results or HTML that could contain sensitive values.
func compressPGAdminAsset(response *http.Response) error {
	request := response.Request
	if request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/static/") && response.StatusCode == http.StatusOK {
		// Only static program assets are reusable across editor iframes. SQL,
		// cookies and authenticated pages must never enter a shared cache.
		response.Header.Set("Cache-Control", "private, max-age=3600")
	}
	kind := strings.Split(response.Header.Get("Content-Type"), ";")[0]
	if request.Method != http.MethodGet || !strings.HasPrefix(request.URL.Path, "/static/") ||
		response.StatusCode != http.StatusOK || response.Header.Get("Content-Encoding") != "" ||
		(kind != "application/javascript" && kind != "text/javascript" && kind != "text/css") {
		return nil
	}
	response.Header.Add("Vary", "Accept-Encoding")
	accepted := false
	for _, encoding := range strings.Split(request.Header.Get("Accept-Encoding"), ",") {
		parts := strings.Split(encoding, ";")
		if strings.TrimSpace(parts[0]) != "gzip" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			if key, value, ok := strings.Cut(strings.TrimSpace(parameter), "="); ok && key == "q" {
				parsed, err := strconv.ParseFloat(value, 64)
				if err != nil || parsed < 0 || parsed > 1 {
					quality = 0
				} else {
					quality = parsed
				}
			}
		}
		accepted = quality > 0
		break
	}
	if !accepted {
		return nil
	}
	original := response.Body
	reader, writer := io.Pipe()
	go func() {
		defer func() { _ = original.Close() }()
		compressed := gzip.NewWriter(writer)
		_, err := io.Copy(compressed, original)
		if closeErr := compressed.Close(); err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
	}()
	response.Body = reader
	response.ContentLength = -1
	response.Header.Del("Content-Length")
	response.Header.Del("Accept-Ranges")
	if etag := response.Header.Get("ETag"); etag != "" && !strings.HasPrefix(etag, "W/") {
		response.Header.Set("ETag", "W/"+etag)
	}
	response.Header.Set("Content-Encoding", "gzip")
	return nil
}

func requestTimeouts(next http.Handler) http.Handler {
	api := http.TimeoutHandler(next, 15*time.Second, `{"error":"request timeout","data_as_of":null}`)
	tools := http.TimeoutHandler(next, 2*time.Minute, `{"error":"operations request timeout","data_as_of":null}`)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/db/") {
			tools.ServeHTTP(w, r)
			return
		}
		api.ServeHTTP(w, r)
	})
}
func registerTools(mux *http.ServeMux, obs *observability) error {
	for _, file := range []string{"openapi.json", "openapi-mocks.json"} {
		body, err := specifications.ReadFile("specifications/" + file)
		if err != nil {
			return err
		}
		mux.HandleFunc("GET /"+file, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(body)
		})
	}
	mux.Handle("GET /metrics", operationsAuth(obs.handler()))
	for _, entry := range []struct {
		prefix, key, fallback string
		strip, auth, grafana  bool
	}{
		{"/docs/", "SWAGGER_URL", "http://swagger-ui:8080", true, false, false},
		{"/db/", "PGADMIN_URL", "http://pgadmin:80", true, true, false},
		{"/grafana/", "GRAFANA_URL", "http://grafana:3000", false, true, true},
		{"/prometheus/", "PROMETHEUS_URL", "http://prometheus:9090", true, true, false},
	} {
		handler, err := toolProxy(env(entry.key, entry.fallback), entry.prefix, entry.strip, entry.grafana)
		if err != nil {
			return fmt.Errorf("%s: %w", entry.key, err)
		}
		if entry.auth {
			handler = operationsAuth(handler)
		}
		mux.Handle(entry.prefix, handler)
		mux.HandleFunc(strings.TrimSuffix(entry.prefix, "/"), func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, entry.prefix, http.StatusTemporaryRedirect)
		})
	}
	return nil
}
