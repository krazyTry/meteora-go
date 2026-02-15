package dammv2

import (
	"context"
	"math/big"

	"github.com/gagliardetto/solana-go/rpc"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

const (
	LiquidityScale = 128
	ScaleOffset    = 64

	BasisPointMax  = 10_000
	FeeDenominator = 1_000_000_000

	MinFeeBps       = 1       // 0.01%
	MinFeeNumerator = 100_000 // 0.01%

	MaxFeeBpsV0       = 5000        // 50%
	MaxFeeNumeratorV0 = 500_000_000 // 50%

	MaxFeeBpsV1       = 9900        // 99%
	MaxFeeNumeratorV1 = 990_000_000 // 99%

	MinCuBuffer = 50_000
	MaxCuBuffer = 200_000

	DynamicFeeFilterPeriodDefault    = 10
	DynamicFeeDecayPeriodDefault     = 120
	DynamicFeeReductionFactorDefault = 5000 // 50%
	BinStepBpsDefault                = 1
	MaxPriceChangeBpsDefault         = 1500 // 15%

	U16Max = 65535

	MaxRateLimiterDurationInSeconds = 43_200
	MaxRateLimiterDurationInSlots   = 108_000

	SplitPositionDenominator = 1_000_000_000
)

var (
	CpAmmProgramID = dammv2gen.ProgramID

	OneQ64         = new(big.Int).Lsh(big.NewInt(1), ScaleOffset)
	MaxExponential = big.NewInt(0x80000)
	MaxU128        = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

	MinSqrtPrice = bigIntFromString("4295048016")
	MaxSqrtPrice = bigIntFromString("79226673521066979257578248091")

	DynamicFeeScalingFactor  = bigIntFromString("100000000000")
	DynamicFeeRoundingOffset = bigIntFromString("99999999999")

	BinStepBpsU128Default = bigIntFromString("1844674407370955")

	U128Max = bigIntFromString("340282366920938463463374607431768211455")
	U64Max  = bigIntFromString("18446744073709551615")
)

var (
	CurrentPoolVersion = PoolVersionV1
	FeePadding         = [3]uint8{0, 0, 0}
)

func bigIntFromString(v string) *big.Int {
	out, ok := new(big.Int).SetString(v, 10)
	if !ok {
		panic("invalid big integer literal")
	}
	return out
}

func CurrentPointForActivation(ctx context.Context, client *rpc.Client, commitment rpc.CommitmentType, activationType ActivationType) *big.Int {
	slot, _ := client.GetSlot(ctx, commitment)
	if activationType == ActivationTypeSlot {
		return new(big.Int).SetUint64(slot)
	}
	bt, _ := client.GetBlockTime(ctx, slot)
	if bt != nil {
		return big.NewInt(int64(*bt))
	}
	return big.NewInt(0)
}
