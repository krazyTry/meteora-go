package u128

import (
	"errors"
	"fmt"
	"math/big"

	binary "github.com/gagliardetto/binary"
)

type Uint128 binary.Uint128

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

func GenUint128FromString(num string) binary.Uint128 {
	u128 := binary.NewUint128LittleEndian()
	if _, err := fmt.Sscan(num, (*Uint128)(u128)); err != nil {
		panic(err)
	}
	return *u128
}
