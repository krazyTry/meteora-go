package u128

import (
	"errors"
	"fmt"
	"math/big"

	binary "github.com/gagliardetto/binary"
)

// Uint128 represents a 128-bit unsigned integer type
// It contains two 64-bit fields:
// - Lo: lower 64 bits of the 128-bit integer
// - Hi: higher 64 bits of the 128-bit integer
type Uint128 binary.Uint128

// Scan implements the fmt.Scanner interface for Uint128
func (u *Uint128) Scan(s fmt.ScanState, ch rune) error {
	i := new(big.Int)
	if err := i.Scan(s, ch); err != nil {
		return err
	} else if i.Sign() < 0 {
		return errors.New("value cannot be negative")
	} else if i.BitLen() > 128 {
		return errors.New("value overflows Uint128")
	}
	u.Lo = i.Uint64()
	u.Hi = i.Rsh(i, 64).Uint64()
	return nil
}

// GenUint128FromString converts a string representation of a large number to u128
func GenUint128FromString(num string) binary.Uint128 {
	u128 := binary.NewUint128LittleEndian()
	if _, err := fmt.Sscan(num, (*Uint128)(u128)); err != nil {
		panic(err)
	}
	return *u128
}
