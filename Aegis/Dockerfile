# Stage 1: Build the Go application
FROM golang:1.25-alpine AS builder

WORKDIR /Aegis # Korrekter WORKDIR für Aegis

# Layer Caching für Go-Module
COPY go.mod go.sum ./
RUN go mod download

# Kopiere den Rest des Codes
COPY . .

RUN mkdir /app
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /app/aegis_executable ./main.go


# --- Stage 2: Finales, minimales Image ---
FROM scratch

# SSL-Zertifikate kopieren
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/aegis_executable /Aegis

# Ports dokumentieren
EXPOSE 8080
EXPOSE 9090

USER 65534

CMD ["/Aegis"]

