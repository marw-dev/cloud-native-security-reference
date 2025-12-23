package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-redis/redis/v8"
)

type CachedResponse struct {
	Status 	int
	Header 	http.Header
	Body	[]byte
}

func GenerateCacheKey(r *http.Request) string {
	if r.Method != http.MethodGet {
		return ""
	}

	key := r.URL.Path + "?" + r.URL.RawQuery
	hasher := sha256.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

type bodyCaptureWriter struct {
	middleware.WrapResponseWriter
	Body bytes.Buffer
}

func (w *bodyCaptureWriter) Write(b []byte) (int, error) {
	w.Body.Write(b)
	return w.WrapResponseWriter.Write(b)
}

func CacheMiddleware(client *redis.Client, ttl time.Duration) func(http.Handler) http.Handler {
	if ttl <= 0 {
		return func(next http.Handler) http.Handler {return next}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			
			cacheKey := GenerateCacheKey(r)
			if cacheKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			cacheData, err := client.Get(context.Background(), cacheKey).Bytes() 
			
			if err == nil { 
				var resp CachedResponse
				if err := json.Unmarshal(cacheData, &resp); err != nil {
					log.Printf("[CACHE ERROR] Deserialisierung fehlgeschlagen: %v", err)
				} else {
					for k, v := range resp.Header {
						for _, h := range v {
							w.Header().Add(k, h)
						}
					}
					w.WriteHeader(resp.Status)
					w.Write(resp.Body)
					log.Printf("[CACHE HIT] %s %s Status: %d, bedient aus Redis", r.Method, r.URL.Path, resp.Status)
					return
				}
			}

			recorder := &bodyCaptureWriter{
				WrapResponseWriter: middleware.NewWrapResponseWriter(w, r.ProtoMajor),
			}
			
			next.ServeHTTP(recorder, r)

			if recorder.Status() >= 200 && recorder.Status() < 300 {
				
				resp := CachedResponse{
					Status: recorder.Status(),
					Header: w.Header(),
					Body:   recorder.Body.Bytes(), 
				}

				serializedData, err := json.Marshal(resp)
				if err != nil {
					log.Printf("[CACHE ERROR] Serialisierung fehlgeschlagen: %v", err)
					return
				}

				client.Set(context.Background(), cacheKey, serializedData, ttl)
				
				log.Printf("[CACHE MISS] %s %s Status: %d, Downstream-Antwort für %v gespeichert.", 
				r.Method, r.URL.Path, resp.Status, ttl)
			}
		})
	}
}