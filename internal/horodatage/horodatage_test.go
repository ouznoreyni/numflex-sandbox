package horodatage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFormatSuitISOInstantDeJava(t *testing.T) {
	base := time.Date(2026, 8, 27, 22, 39, 23, 0, time.UTC)
	cas := []struct {
		ns     int
		attend string
	}{
		{0, "2026-08-27T22:39:23Z"},
		{583_000_000, "2026-08-27T22:39:23.583Z"},
		{580_000_000, "2026-08-27T22:39:23.580Z"}, // pas « .58Z »
		{583_043_000, "2026-08-27T22:39:23.583043Z"},
		{583_043_149, "2026-08-27T22:39:23.583043149Z"},
		{5_000, "2026-08-27T22:39:23.000005Z"},
	}
	for _, c := range cas {
		require.Equal(t, c.attend, Format(base.Add(time.Duration(c.ns))))
	}
}

func TestFormatConvertitEnUTC(t *testing.T) {
	dakar := time.FixedZone("GMT+1", 3600)
	require.Equal(t, "2026-08-27T21:39:23.583Z",
		Format(time.Date(2026, 8, 27, 22, 39, 23, 583_000_000, dakar)))
}

func TestInstantSeSerialiseEnChaine(t *testing.T) {
	b, err := json.Marshal(map[string]any{
		"d": Instant(time.Date(2026, 8, 27, 22, 39, 23, 583_043_149, time.UTC)),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"d":"2026-08-27T22:39:23.583043149Z"}`, string(b))
}

func TestFraisRetrouveLaValeurEcriteParLaRequete(t *testing.T) {
	ctx := context.WithValue(context.Background(), Cle, Nouveau())
	ecrit := time.Date(2026, 8, 27, 22, 39, 23, 583_043_149, time.UTC)
	Marquer(ctx, ecrit)

	// Postgres rend la microseconde : les nanosecondes sont perdues.
	relu := ecrit.Truncate(time.Microsecond)
	frais, ok := Frais(ctx, relu)
	require.True(t, ok)
	require.Equal(t, ecrit, frais)

	_, ok = Frais(ctx, relu.Add(time.Second))
	require.False(t, ok, "un horodatage d'une autre requête n'est pas frais")
}

func TestSansRegistreRienNEstFrais(t *testing.T) {
	ctx := context.Background()
	Marquer(ctx, time.Now()) // ne panique pas
	_, ok := Frais(ctx, time.Now())
	require.False(t, ok)
}
