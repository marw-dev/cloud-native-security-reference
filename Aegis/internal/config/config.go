package config

type RateLimitConfig struct {
	Limit  uint32 `yaml:"limit" json:"limit"`
	Window string `yaml:"window" json:"window"`
}

type CircuitBreakerConfig struct {
	FailureThreshold int    `yaml:"failure_threshold" json:"failure_threshold"`
	OpenTimeout      string `yaml:"open_timeout" json:"open_timeout"`
}
type RouteConfig struct {
	Path           string               `yaml:"path" json:"path"`
	TargetURL      string               `yaml:"target_url" json:"target_url"`
	RequiredRoles  []string             `yaml:"required_roles,omitempty" json:"required_roles,omitempty"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`
	CacheTTL       string               `yaml:"cache_ttl,omitempty" json:"cache_ttl,omitempty"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker,omitempty" json:"circuit_breaker,omitempty"`

	ProxyTimeout string `yaml:"proxy_timeout,omitempty" json:"proxy_timeout,omitempty"`

	WebhookSecret          string `yaml:"webhook_secret,omitempty" json:"webhook_secret,omitempty"`
	WebhookSignatureHeader string `yaml:"webhook_signature_header,omitempty" json:"webhook_signature_header,omitempty"`
}

type CorsConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins,omitempty" json:"allowed_origins,omitempty"`
}

type GatewayConfig struct {
	Routes []RouteConfig `yaml:"routes" json:"routes"` // Wichtig: JSON-Tag
	Port   int           `yaml:"port" json:"port"`

	ContextMap    map[string]string `json:"context_map"` // Map[Host] -> ProjectID
	ContextMapURL string            `json:"-"`           // Wird aus Env geladen, nicht API
	AdminHost     string            `json:"-"`           // z.B. athena.deine-firma.de

	JwtPublicKeyPath string `yaml:"jwt_public_key_path" json:"jwt_public_key_path"`

	Cors        CorsConfig `yaml:"cors,omitempty" json:"cors,omitempty"`
	MetricsPort int        `yaml:"metrics_port,omitempty" json:"metrics_port,omitempty"`
	RedisAddr   string     `yaml:"redis_addr" json:"redis_addr"`

	TLSCertPath string `yaml:"tls_cert_path,omitempty" json:"tls_cert_path,omitempty"`
	TLSKeyPath  string `yaml:"tls_key_path,omitempty" json:"tls_key_path,omitempty"`
}