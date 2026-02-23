package helpers

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math/big"
	"reflect"

	binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
	"github.com/shopspring/decimal"
)

func ConvertToLamports(amount string, tokenDecimal int32) (*big.Int, error) {
	value, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	value = value.Mul(decimal.New(1, tokenDecimal))
	return FromDecimalToBig(value), nil
}

func FromDecimalToBig(value decimal.Decimal) *big.Int {
	return value.Truncate(0).BigInt()
}

// func CreateProgramAccountFilter(owner solanago.PublicKey, offset uint64) []rpc.RPCFilter {
// 	return []rpc.RPCFilter{
// 		{Memcmp: &rpc.RPCFilterMemcmp{Offset: offset, Bytes: owner[:]}},
// 	}
// }

// Filter represents a filter for querying accounts by owner and offset
type Filter struct {
	Owner  solana.PublicKey // Account owner to filter by
	Offset uint64           // Offset for pagination
}

func discriminator(name string) []byte {
	hash := sha256.Sum256([]byte("account:" + name))
	var out [8]byte
	copy(out[:], hash[:8])
	return out[:]
}

// ComputeStructOffset gets the offset position of an object in a struct
func ComputeStructOffset(x any, o string) uint64 {
	t := reflect.TypeOf(x).Elem()
	fields := make([]reflect.StructField, 0)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == o {
			break
		}
		fields = append(fields, f)
	}

	newType := reflect.StructOf(fields)
	newValue := reflect.New(newType).Elem()

	buf__ := new(bytes.Buffer)
	enc__ := binary.NewBorshEncoder(buf__)
	enc__.Encode(newValue.Interface())

	// instruction discriminators offset = 8
	return uint64(buf__.Len()) + 8
}

func CreateProgramAccountFilter(key string, filter *Filter) []rpc.RPCFilter {
	var filters []rpc.RPCFilter
	filters = append(filters, rpc.RPCFilter{
		Memcmp: &rpc.RPCFilterMemcmp{
			Offset: 0,
			Bytes:  discriminator(key),
		},
	})

	if filter != nil {
		filters = append(filters, rpc.RPCFilter{
			Memcmp: &rpc.RPCFilterMemcmp{
				Offset: filter.Offset,
				Bytes:  filter.Owner[:],
			},
		})
	}

	return filters
}

func IsNativeSol(mint solanago.PublicKey) bool {
	return mint.Equals(NativeMint)
}

func IsDefaultLockedVesting(lockedVesting struct {
	AmountPerPeriod                *big.Int
	CliffDurationFromMigrationTime *big.Int
	Frequency                      *big.Int
	NumberOfPeriod                 *big.Int
	CliffUnlockAmount              *big.Int
}) bool {
	return lockedVesting.AmountPerPeriod.Sign() == 0 &&
		lockedVesting.CliffDurationFromMigrationTime.Sign() == 0 &&
		lockedVesting.Frequency.Sign() == 0 &&
		lockedVesting.NumberOfPeriod.Sign() == 0 &&
		lockedVesting.CliffUnlockAmount.Sign() == 0
}

func BpsToFeeNumerator(bps uint64) *big.Int {
	return new(big.Int).Div(new(big.Int).Mul(new(big.Int).SetUint64(bps), big.NewInt(shared.FeeDenominator)), big.NewInt(shared.MaxBasisPoint))
}

func FeeNumeratorToBps(feeNumerator *big.Int) uint64 {
	return new(big.Int).Div(new(big.Int).Mul(feeNumerator, big.NewInt(shared.MaxBasisPoint)), big.NewInt(shared.FeeDenominator)).Uint64()
}

// BigIntToU64 converts a non-negative big.Int to uint64 with bounds check.
func BigIntToU64(v *big.Int) (uint64, error) {
	if v == nil {
		return 0, nil
	}
	if v.Sign() < 0 {
		return 0, errors.New("value must be non-negative")
	}
	if v.BitLen() > 64 {
		return 0, errors.New("value overflows uint64")
	}
	return v.Uint64(), nil
}
