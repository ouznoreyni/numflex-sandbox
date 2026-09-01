package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

var motifMSISDN = regexp.MustCompile(`^[0-9]{9}$`)

func (d *Deps) routesOTP(g *gin.RouterGroup) {
	g.POST("/otp/send", d.postOTPSend)
	g.POST("/otp/verify", d.postOTPVerify)
}

type reqOTPSend struct {
	Numero string `json:"numero"`
}

type reqOTPVerify struct {
	Numero  string `json:"numero"`
	OtpCode string `json:"otpCode"`
}

func (d *Deps) postOTPSend(c *gin.Context) {
	var req reqOTPSend
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, entity.InvalidJSONFormat())
		return
	}
	if !motifMSISDN.MatchString(req.Numero) {
		d.R.Fail(c, entity.Validation(entity.FieldFault{
			ObjectName: "otpSendDTO", Field: "numero",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		}))
		return
	}

	maintenant := time.Now()
	_, err := d.DB.Pool.Exec(c,
		`INSERT INTO otp (numero, code, expire_a, tentatives, consomme, cree_le)
		 VALUES ($1,$2,$3,0,false,$4)
		 ON CONFLICT (numero) DO UPDATE
		   SET code = EXCLUDED.code, expire_a = EXCLUDED.expire_a,
		       tentatives = 0, consomme = false, cree_le = EXCLUDED.cree_le`,
		req.Numero, d.Cfg.OTPStaticCode, maintenant.Add(d.Cfg.OTPTTL), maintenant)
	if err != nil {
		d.R.Fail(c, entity.InternalError("enregistrement de l'OTP"))
		return
	}

	// Le sandbox n'envoie pas de SMS : le code est statique et journalisé.
	// La réponse acquitte la soumission, pas la remise (ANO-021).
	d.R.OKSansData(c, http.StatusOK, "OTP envoyé avec succès")
}

func (d *Deps) postOTPVerify(c *gin.Context) {
	var req reqOTPVerify
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, entity.InvalidJSONFormat())
		return
	}
	if e := d.verifierOTP(c, req.Numero, req.OtpCode); e != nil {
		d.R.Fail(c, e)
		return
	}
	d.R.OKSansData(c, http.StatusOK, "Code OTP vérifié avec succès")
}

// verifierOTP pré-vérifie sans consommer (TC-021) : le code reste utilisable pour
// créer la demande. Seules les tentatives ratées sont décomptées.
func (d *Deps) verifierOTP(ctx context.Context, numero, code string) *entity.Fault {
	var stocke string
	var expireA time.Time
	var tentatives int
	var consomme bool

	err := d.DB.Pool.QueryRow(ctx,
		`SELECT code, expire_a, tentatives, consomme FROM otp WHERE numero = $1`, numero).
		Scan(&stocke, &expireA, &tentatives, &consomme)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.OTPAbsent()
	}
	if err != nil {
		return entity.InternalError("lecture de l'OTP")
	}

	if consomme {
		return entity.OTPAlreadyUsed()
	}
	if tentatives >= d.Cfg.OTPMaxAttempts {
		return entity.OTPMaxAttempts()
	}
	if time.Now().After(expireA) {
		return entity.OTPExpired()
	}
	if code != stocke {
		// L'échec de cet incrément ne peut pas être avalé : sans lui, la limite
		// de trois tentatives cesse silencieusement de s'appliquer.
		if _, err := d.DB.Pool.Exec(ctx,
			`UPDATE otp SET tentatives = tentatives + 1 WHERE numero = $1`, numero); err != nil {
			return entity.InternalError("incrément des tentatives OTP")
		}
		return entity.OTPInvalid()
	}
	return nil
}

func (d *Deps) consommerOTP(ctx context.Context, numero string) error {
	_, err := d.DB.Pool.Exec(ctx, `UPDATE otp SET consomme = true WHERE numero = $1`, numero)
	return err
}
