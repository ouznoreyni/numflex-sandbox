package config

import (
	"fmt"
	"os"
	"strconv"
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
	EtapeTimeout          time.Duration
	EngineTick            time.Duration
	ConvergenceMin        time.Duration
	ConvergenceMax        time.Duration
	CompletionLatency     time.Duration
	ClockSkew             time.Duration
	OTPStaticCode         string
	OTPTTL                time.Duration
	OTPMaxAttempts        int
	ReverseAutoValidation time.Duration
}

func Load() (*Config, error) {
	c := &Config{
		Port:          str("PORT", "8080"),
		DatabaseURL:   str("DATABASE_URL", ""),
		JWTSecret:     str("JWT_SECRET", "numflex-sandbox-dev-secret"),
		Fidelity:      Fidelity(str("FIDELITY", string(FidelityReal))),
		OTPStaticCode: str("OTP_STATIC_CODE", "123456"),
	}

	var err error
	if c.JWTTTL, err = dur("JWT_TTL_HOURS", 24, time.Hour); err != nil {
		return nil, err
	}
	if c.EtapeTimeout, err = dur("ETAPE_TIMEOUT_SECONDS", 349, time.Second); err != nil {
		return nil, err
	}
	if c.EngineTick, err = dur("ENGINE_TICK_SECONDS", 10, time.Second); err != nil {
		return nil, err
	}
	if c.ConvergenceMin, err = dur("CONVERGENCE_MIN_SECONDS", 60, time.Second); err != nil {
		return nil, err
	}
	if c.ConvergenceMax, err = dur("CONVERGENCE_MAX_SECONDS", 360, time.Second); err != nil {
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

func str(clef, defaut string) string {
	if v, ok := os.LookupEnv(clef); ok && v != "" {
		return v
	}
	return defaut
}

func num(clef string, defaut int) (int, error) {
	v, ok := os.LookupEnv(clef)
	if !ok || v == "" {
		return defaut, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s : entier attendu, reçu %q", clef, v)
	}
	return n, nil
}

func dur(clef string, defaut int, unite time.Duration) (time.Duration, error) {
	n, err := num(clef, defaut)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("%s ne peut être négatif", clef)
	}
	return time.Duration(n) * unite, nil
}
