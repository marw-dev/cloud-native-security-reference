# Cloud-Native Security Reference Architecture
> **A production-grade reference architecture for secure Microservices in Go.**
> Featuring HashiCorp Vault (AppRole), OpenTelemetry (LGTM), RS256 Auth & Circuit Breakers.

## System Architecture

```mermaid
graph TD
    %% Styles definieren
    classDef client fill:#f9f9f9,stroke:#333,stroke-width:2px;
    classDef go fill:#00ADD8,stroke:#333,stroke-width:2px,color:white;
    classDef db fill:#e1e1e1,stroke:#333,stroke-width:2px;
    classDef sec fill:#e05d44,stroke:#333,stroke-width:2px,color:white;
    classDef obs fill:#4bb543,stroke:#333,stroke-width:2px,color:white;

    %% Externe Akteure
    User((Client / User)):::client
    Admin((Admin)):::client

    %% Frontend Bereich
    subgraph "Public Zone"
        UI["AthenaUI<br>(Vanilla JS SPA)"]:::client
    end

    %% Backend Bereich
    subgraph "Private Network (Docker Compose)"
        
        %% Haupt-Services
        Aegis["Aegis API Gateway<br>(Go Proxy & Routing)"]:::go
        Athena["Athena Identity Service<br>(Go Auth Provider)"]:::go
        
        %% Infrastruktur / Daten
        Redis[("Redis<br>Rate Limiter & Cache")]:::db
        MySQL[("MySQL<br>User & Project DB")]:::db
        Vault["HashiCorp Vault<br>Secrets & PKI Engine"]:::sec
        
        %% Observability Stack (LGTM)
        subgraph "Observability & Telemetry"
            OTel["OpenTelemetry Collector"]:::obs
            Loki["Loki<br>(Logs)"]:::obs
            Tempo["Tempo<br>(Traces)"]:::obs
            Prom["Prometheus<br>(Metrics)"]:::obs
            Grafana["Grafana<br>(Visualisierung)"]:::obs
        end
    end

    %% Verbindungen (Flow)
    User -- "HTTPS Request" --> Aegis
    Admin -- "Manage" --> UI
    UI -- "API Calls" --> Aegis

    %% Interne Logik Aegis
    Aegis -- "1. Check Limits" --> Redis
    Aegis -- "2. Validate Session" --> Athena
    Aegis -- "Circuit Breaker" --> Athena

    %% Interne Logik Athena
    Athena -- "AppRole Auth & Secrets" --> Vault
    Athena -- "CRUD Data" --> MySQL
    Athena -- "Sign JWT (RS256)" --> Vault

    %% Telemetrie Streams
    Aegis -.->|"gRPC OTLP"| OTel
    Athena -.->|"gRPC OTLP"| OTel

    OTel --> Loki
    OTel --> Tempo
    OTel --> Prom
    Prom --> Grafana
    Loki --> Grafana
    Tempo --> Grafana

    %% Legende
    linkStyle default stroke-width:2px,fill:none,stroke:gray;
