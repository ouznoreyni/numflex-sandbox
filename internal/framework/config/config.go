package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
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

	// Docs registers /swagger.html, /openapi.yaml and /openapi.json at the
	// root — outside /api/gateway/v1, which keeps exactly its 33 routes
	// either way. True by default so that running the standalone image needs
	// no argument at all; DOCS_ENABLED=false gives back the platform's exact
	// surface. It is a boolean rather than an empty DocsDir because an empty
	// value counts as absent everywhere in this configuration.
	Docs bool

	// DocsDir is where those three files are looked up, walking up from the
	// working directory. The routes appear only if the folder is actually
	// there, so the scratch-based `runtime` image — which ships none —
	// registers nothing even with Docs true.
	DocsDir string

	// PoolPerOperator is how many never-ported numbers ORANGE and YAS each
	// get at seed time, spread over their eight ranges — the pool a porting
	// consumes, one number per successful cycle. It is the one setting that
	// costs real time and disk, which is why the default is a hundred
	// thousand per range rather than a full one: DefaultPoolPerOperator
	// starts in about twenty seconds, FullNumbers in four and a half
	// minutes. EXPRESSO and the already-ported ranges keep their fixed size,
	// being rejection material rather than something to consume.
	PoolPerOperator int

	// FullNumbers fills every portable range whole — its million numbers,
	// 000000 to 999999 — so that any well-formed number of a range exists.
	// It is a shortcut on PoolPerOperator's DEFAULT, not an override: an
	// explicit POOL_NUMBERS_PER_OPERATOR still wins, so the two can never
	// contradict each other.
	FullNumbers bool
}

// DefaultPoolPerOperator is the pool seeded when nothing is asked: a hundred
// thousand numbers per range, eight ranges per operator. Enough that no
// exploration exhausts it, small enough that a container without a
// persistent volume starts in seconds — FULL_NUMBERS=true is there for the
// day the whole range is wanted.
const DefaultPoolPerOperator = 800_000

func Load() (*Config, error) {
	c := &Config{
		Port:          str("PORT", "8080"),
		DatabaseURL:   str("DATABASE_URL", ""),
		JWTSecret:     str("JWT_SECRET", "numflex-sandbox-dev-secret"),
		Fidelity:      Fidelity(str("FIDELITY", string(FidelityReal))),
		OTPStaticCode: str("OTP_STATIC_CODE", "123456"),
		DocsDir:       str("DOCS_DIR", "docs"),
	}

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
	// FULL_NUMBERS is read first: it decides the pool's default, which the
	// line below then lets POOL_NUMBERS_PER_OPERATOR override.
	if c.FullNumbers, err = boolean("FULL_NUMBERS", false); err != nil {
		return nil, err
	}
	pool := DefaultPoolPerOperator
	if c.FullNumbers {
		pool = seed.UnportedRangesPerOperator * seed.MaxPerRange
	}
	if c.PoolPerOperator, err = num("POOL_NUMBERS_PER_OPERATOR", pool); err != nil {
		return nil, err
	}
	if c.Docs, err = boolean("DOCS_ENABLED", true); err != nil {
		return nil, err
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.Fidelity != FidelityReal && c.Fidelity != FidelityContract {
		return nil, fmt.Errorf("FIDELITY must be %q or %q, got %q",
			FidelityReal, FidelityContract, c.Fidelity)
	}
	if c.ConvergenceMax < c.ConvergenceMin {
		return nil, fmt.Errorf("CONVERGENCE_MAX_SECONDS ne peut être inférieur à CONVERGENCE_MIN_SECONDS")
	}
	if c.PoolPerOperator < seed.UnportedRangesPerOperator {
		return nil, fmt.Errorf("POOL_NUMBERS_PER_OPERATOR must be at least %d, one per range",
			seed.UnportedRangesPerOperator)
	}
	if c.PoolPerOperator > seed.UnportedRangesPerOperator*seed.MaxPerRange {
		return nil, fmt.Errorf("POOL_NUMBERS_PER_OPERATOR cannot exceed %d: a range holds %d numbers at most",
			seed.UnportedRangesPerOperator*seed.MaxPerRange, seed.MaxPerRange)
	}
	if c.EngineTick <= 0 {
		return nil, fmt.Errorf("ENGINE_TICK_SECONDS must be strictly positive")
	}
	return c, nil
}

func str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func num(key string, def int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: integer expected, got %q", key, v)
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
		return false, fmt.Errorf("%s: boolean expected, got %q", key, v)
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
