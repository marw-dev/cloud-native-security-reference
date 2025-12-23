package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
)

func WebhookSignatureMiddleware(secretKey string, headerName string) func(http.Handler) http.Handler {
	if secretKey == "" {
		log.Println("WARN: Webhook Signature Key fehlt. Validierung deaktiviert.")
		return func(next http.Handler) http.Handler { return next }
	}

	if headerName == "" {
		headerName = "X-Webhook-Signature"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost && r.Method != http.MethodPut {
				next.ServeHTTP(w, r)
				return
			}

			signatureHeader := r.Header.Get(headerName)
			if signatureHeader == "" {
				http.Error(w, "Fehlende Webhook Signatur im Header: "+headerName, http.StatusUnauthorized)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("WEBHOOK ERROR: Lesen des Bodies fehlgeschlagen: %v", err)
				http.Error(w, "Interner Fehler beim Lesen des Bodies", http.StatusInternalServerError)
				return
			}
			
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			expectedSignature := generateHMAC(body, secretKey)
			
			if !hmac.Equal([]byte(expectedSignature), []byte(signatureHeader)) {
				http.Error(w, "Ungültige Webhook Signatur", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func generateHMAC(body []byte, secret string) string {
	hasher := hmac.New(sha256.New, []byte(secret))
	hasher.Write(body)
	return hex.EncodeToString(hasher.Sum(nil))
}
