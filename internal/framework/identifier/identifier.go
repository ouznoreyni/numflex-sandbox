// Package identifier produces identifiers shaped like a MongoDB ObjectId —
// 24 hexadecimal characters. The NumFlex UAT platform returns this format;
// the v2 guide only ever shows illustrative examples like "dem-abc123",
// never actually observed.
package identifier

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"sync/atomic"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

var (
	processBytes [5]byte
	counter      uint32
)

func init() {
	if _, err := crand.Read(processBytes[:]); err != nil {
		panic("identifier : source d'aléa indisponible : " + err.Error())
	}
	var seedBytes [4]byte
	if _, err := crand.Read(seedBytes[:]); err != nil {
		panic("identifier : source d'aléa indisponible : " + err.Error())
	}
	counter = binary.BigEndian.Uint32(seedBytes[:])
}

// New returns an ObjectId: 4 bytes of timestamp, 5 process-specific bytes,
// 3 bytes of incremental counter.
func New() string {
	var b [12]byte
	binary.BigEndian.PutUint32(b[0:4], uint32(time.Now().Unix()))
	copy(b[4:9], processBytes[:])
	c := atomic.AddUint32(&counter, 1)
	b[9] = byte(c >> 16)
	b[10] = byte(c >> 8)
	b[11] = byte(c)
	return hex.EncodeToString(b[:])
}

// Generator is the port.IDGenerator implementation backed by New.
type Generator struct{}

// NewGenerator returns a ready-to-use Generator.
func NewGenerator() Generator { return Generator{} }

func (Generator) NewID() string { return New() }

var _ port.IDGenerator = Generator{}
