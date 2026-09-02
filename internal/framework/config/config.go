package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Fidelity string

const (
	FidelityReal     Fidelity = "real"
	FidelityContract Fidelity = "contract"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	JWTSecret             string
	JWTTTL                time.Duration
	Fidelity              Fidelity
	StepTimeout           time.Duration
	EngineTick            time.Duration
	ConvergenceMin        time.Duration
	ConvergenceMax        time.Duration
	CompletionLatency     time.Duration
	ClockSkew             time.Duration
	OTPStaticCode         string
	OTPTTL                time.Duration
	OTPMaxAttempts        int
	ReverseAutoValidation time.Duration

	// SandboxAdmin opens /api/sandbox/v1 — the test-data purge. Outside the
	// ARTP contract, so false by default: at false the route is not
	// registered at all and answers 404, like any unknown path. The gateway,
	// on the other hand, keeps its 33 routes either way.
	SandboxAdmin bool

	// CORSAllowedOrigins is a sandbox convenience, not a trait of the
	// contract: it exists only so that a page served on another port —
	// Swagger, a back-office in development — can call the API from a
	// browser. The default is `*`, every origin allowed, so it works without
	// any configuration. The real gateway, consumed server-to-server, emits
	// no CORS header at all: setting CORS_ALLOWED_ORIGINS to empty restores
	// that behaviour.
	CORSAllowedOrigins []string
}

func Load() (*Config, error) {
	c := &Config{
		Port:          str("PORT", "8080"),
		DatabaseURL:   str("DATABASE_URL", ""),
		JWTSecret:     str("JWT_SECRET", "numflex-sandbox-dev-secret"),
		Fidelity:      Fidelity(str("FIDELITY", string(FidelityReal))),
		OTPStaticCode: str("OTP_STATIC_CODE", "123456"),
	}

	// The one variable where an empty string differs from being unset,
	// because here the two carry opposite meanings: not set, CORS is open to
	// every origin; set empty, it is switched off.
	origins := "*"
	if v, ok := os.LookupEnv("CORS_ALLOWED_ORIGINS"); ok {
		origins = v
	}
	c.CORSAllowedOrigins = splitList(origins)

	var err error
	if c.JWTTTL, err = dur("JWT_TTL_HOURS", 24, time.Hour); err != nil {
		return nil, err
	}
	if c.StepTimeout, err = dur("STEP_TIMEOUT_SECONDS", 349, time.Second); err != nil {
		return nil, err
	}
	if c.EngineTick, err = dur("ENGINE_TICK_SECONDS", 10, time.Second); err != nil {
		return nil, err
	}
	if c.ConvergenceMin, err = dur("CONVERGENCE_MIN_SECONDS", 0, time.Second); err != nil {
		return nil, err
	}
	if c.ConvergenceMax, err = dur("CONVERGENCE_MAX_SECONDS", 0, time.Second); err != nil {
		return nil, err
	}
	if c.CompletionLatency, err = dur("COMPLETION_LATENCY_MS", 30500, time.Millisecond); err != nil {
		return nil, err
	}
	if c.ClockSkew, err = dur("CLOCK_SKEW_SECONDS", 540, time.Second); err != nil {
		return nil, err
	}
	if c.OTPTTL, err = dur("OTP_TTL_SECONDS", 300, time.Second); err != nil {
		return nil, err
	}
	if c.ReverseAutoValidation, err = dur("REVERSE_AUTO_VALIDATION_SECONDS", 0, time.Second); err != nil {
		return nil, err
	}
	if c.OTPMaxAttempts, err = num("OTP_MAX_ATTEMPTS", 3); err != nil {
		return nil, err
	}
	if c.SandboxAdmin, err = boolean("SANDBOX_ADMIN", false); err != nil {
		return nil, err
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL est obligatoire")
	}
	if c.Fidelity != FidelityReal && c.Fidelity != FidelityContract {
		return nil, fmt.Errorf("FIDELITY doit valoir %q ou %q, reçu %q",
			FidelityReal, FidelityContract, c.Fidelity)
	}
	if c.ConvergenceMax < c.ConvergenceMin {
		return nil, fmt.Errorf("CONVERGENCE_MAX_SECONDS ne peut être inférieur à CONVERGENCE_MIN_SECONDS")
	}
	if c.EngineTick <= 0 {
		return nil, fmt.Errorf("ENGINE_TICK_SECONDS doit être strictement positif")
	}
	return c, nil
}

func str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// splitList splits a comma-separated value, ignoring empty entries — "a, ,b"
// gives ["a" "b"], "" gives nil.
func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func num(key string, def int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s : entier attendu, reçu %q", key, v)
	}
	return n, nil
}

func boolean(key string, def bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s : booléen attendu, reçu %q", key, v)
	}
	return b, nil
}

func dur(key string, def int, unit time.Duration) (time.Duration, error) {
	n, err := num(key, def)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("%s ne peut être négatif", key)
	}
	return time.Duration(n) * unit, nil
}
