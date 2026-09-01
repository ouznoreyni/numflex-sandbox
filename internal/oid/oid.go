// Package oid produit des identifiants au format ObjectId MongoDB — 24 caractères
// hexadécimaux. La plateforme NumFlex de recette renvoie ce format ; le guide v2
// n'affiche que des exemples illustratifs du type "dem-abc123", jamais rencontrés.
package oid

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"sync/atomic"
	"time"
)

var (
	processus [5]byte
	compteur  uint32
)

func init() {
	if _, err := crand.Read(processus[:]); err != nil {
		panic("oid : source d'aléa indisponible : " + err.Error())
	}
	var amorce [4]byte
	if _, err := crand.Read(amorce[:]); err != nil {
		panic("oid : source d'aléa indisponible : " + err.Error())
	}
	compteur = binary.BigEndian.Uint32(amorce[:])
}

// New retourne un ObjectId : 4 octets d'horodatage, 5 octets propres au processus,
// 3 octets de compteur incrémental.
func New() string {
	var b [12]byte
	binary.BigEndian.PutUint32(b[0:4], uint32(time.Now().Unix()))
	copy(b[4:9], processus[:])
	c := atomic.AddUint32(&compteur, 1)
	b[9] = byte(c >> 16)
	b[10] = byte(c >> 8)
	b[11] = byte(c)
	return hex.EncodeToString(b[:])
}
