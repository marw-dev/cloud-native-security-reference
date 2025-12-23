# Stage 1: Build the Go application
FROM golang:1.25-alpine AS builder

WORKDIR /athena # Korrekter WORKDIR für Athena

# Layer Caching für Go-Module
COPY go.mod go.sum ./
RUN go mod download

# Kopiere den Rest des Codes
COPY . .

RUN mkdir /app
# Beachte: Der Einstiegspunkt für Athena ist ./cmd/main.go
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /app/athena_executable ./cmd/main.go


# --- Stage 2: Finales, minimales Image ---
FROM scratch

# SSL-Zertifikate kopieren
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/athena_executable /athena

# Migrationen kopieren
COPY ./migrations /migrations

# Ports dokumentieren
EXPOSE 8081

# Als unprivilegierter Benutzer ausführen
USER 65534

CMD ["/athena"]