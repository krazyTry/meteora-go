package helpers

import (
	"errors"
	"math"
	"math/big"

	dbcmath "github.com/krazyTry/meteora-go/dynamic_bonding_curve/math"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

// GetQuoteReserveFromNextSqrtPrice is the quote the curve holds once price has moved from the start to nextSqrtPrice.
func GetQuoteReserveFromNextSqrtPrice(nextSqrtPrice, sqrtStartPrice *big.Int, curve []shared.LiquidityDistributionParameters) (*big.Int, error) {
	total := big.NewInt(0)
	lower := new(big.Int).Set(sqrtStartPrice)
	for _, point := range LiveCurvePoints(curve) {
		upper := point.SqrtPrice.BigInt()
		if nextSqrtPrice.Cmp(lower) > 0 {
			capUpper := upper
			if nextSqrtPrice.Cmp(capUpper) < 0 {
				capUpper = nextSqrtPrice
			}
			delta, err := dbcmath.GetDeltaAmountQuoteUnsigned(lower, capUpper, point.Liquidity.BigInt(), shared.RoundingUp)
			if err != nil {
				return nil, err
			}
			total.Add(total, delta)
		}
		lower = upper
	}
	return total, nil
}

// GetMigrationQuoteAmountFromMigrationBase is the quote that sizes a migration base amount, rounded down.
func GetMigrationQuoteAmountFromMigrationBase(migrationBaseAmount, sqrtMigrationPrice *big.Int, migrationOption shared.MigrationOption, migratedCollectFeeMode shared.MigratedCollectFeeMode) (*big.Int, error) {
	if migrationOption == shared.MigrationOptionMetDamm || migratedCollectFeeMode == shared.MigratedCollectFeeModeCompounding {
		return new(big.Int).Rsh(new(big.Int).Mul(new(big.Int).Mul(migrationBaseAmount, sqrtMigrationPrice), sqrtMigrationPrice), 128), nil
	}
	if migrationOption == shared.MigrationOptionMetDammV2 {
		liquidity, err := dbcmath.GetInitialLiquidityFromDeltaBase(migrationBaseAmount, shared.MaxSqrtPrice, sqrtMigrationPrice)
		if err != nil {
			return nil, err
		}
		return dbcmath.GetDeltaAmountQuoteUnsigned(shared.MinSqrtPrice, sqrtMigrationPrice, liquidity, shared.RoundingDown)
	}
	return nil, errors.New(shared.ErrInvalidMigrationOption)
}

// GetProtocolMigrationFee splits the migration fee. DAMM v2 concentrated liquidity rounds the base fee down.
func GetProtocolMigrationFee(depositBaseAmount, depositQuoteAmount, migrationSqrtPrice *big.Int, migrationFeeBps uint64, migrationOption shared.MigrationOption, migratedCollectFeeMode shared.MigratedCollectFeeMode) (*big.Int, *big.Int, error) {
	quoteFee, err := MulDiv(depositQuoteAmount, new(big.Int).SetUint64(migrationFeeBps), big.NewInt(shared.MaxBasisPoint), shared.RoundingDown)
	if err != nil {
		return nil, nil, err
	}
	if migrationOption == shared.MigrationOptionMetDamm || migratedCollectFeeMode == shared.MigratedCollectFeeModeCompounding {
		baseFee, err := MulDiv(depositBaseAmount, new(big.Int).SetUint64(migrationFeeBps), big.NewInt(shared.MaxBasisPoint), shared.RoundingDown)
		if err != nil {
			return nil, nil, err
		}
		return baseFee, quoteFee, nil
	}
	if migrationOption == shared.MigrationOptionMetDammV2 {
		liquidity, err := dbcmath.GetInitialLiquidityFromDeltaQuote(quoteFee, shared.MinSqrtPrice, migrationSqrtPrice)
		if err != nil {
			return nil, nil, err
		}
		baseFee, err := dbcmath.GetDeltaAmountBaseUnsigned(migrationSqrtPrice, shared.MaxSqrtPrice, liquidity, shared.RoundingDown)
		if err != nil {
			return nil, nil, err
		}
		return baseFee, quoteFee, nil
	}
	return nil, nil, errors.New(shared.ErrInvalidMigrationOption)
}

// TransferFeeMigrationPositionSplit is the second DAMM v2 position created for a transfer-fee migration.
type TransferFeeMigrationPositionSplit struct {
	Numerator               *big.Int
	SecondPositionLiquidity *big.Int
	SecondPosition          string
}

// GetTransferFeeMigrationPositionSplit moves liquidity into the smaller of the partner and creator shares.
func GetTransferFeeMigrationPositionSplit(distributableLiquidity *big.Int, partnerLiquidityPercentage, partnerPermanentLockedLiquidityPercentage, partnerVestingPercentage uint8) (TransferFeeMigrationPositionSplit, error) {
	for _, percentage := range []uint8{partnerLiquidityPercentage, partnerPermanentLockedLiquidityPercentage, partnerVestingPercentage} {
		if percentage > 100 {
			return TransferFeeMigrationPositionSplit{}, errors.New("liquidity percentage must be an integer from 0 to 100")
		}
	}
	partnerPercentage := int(partnerLiquidityPercentage) + int(partnerPermanentLockedLiquidityPercentage) + int(partnerVestingPercentage)
	if partnerPercentage > 100 {
		return TransferFeeMigrationPositionSplit{}, errors.New("Invalid liquidity percentage")
	}
	share := func(percentage uint8) *big.Int {
		return new(big.Int).Div(new(big.Int).Mul(distributableLiquidity, big.NewInt(int64(percentage))), big.NewInt(100))
	}
	partnerTotal := new(big.Int).Add(share(partnerLiquidityPercentage), share(partnerPermanentLockedLiquidityPercentage))
	partnerTotal.Add(partnerTotal, share(partnerVestingPercentage))
	creatorTotal := new(big.Int).Sub(distributableLiquidity, partnerTotal)
	secondPosition := "partner"
	percentage := partnerPercentage
	if partnerTotal.Cmp(creatorTotal) > 0 {
		secondPosition = "creator"
		percentage = 100 - partnerPercentage
	}
	denominator := big.NewInt(1_000_000_000)
	numerator := new(big.Int).Mul(new(big.Int).Div(denominator, big.NewInt(100)), big.NewInt(int64(percentage)))
	secondLiquidity, err := MulDiv(distributableLiquidity, numerator, denominator, shared.RoundingDown)
	if err != nil {
		return TransferFeeMigrationPositionSplit{}, err
	}
	return TransferFeeMigrationPositionSplit{
		Numerator:               numerator,
		SecondPositionLiquidity: secondLiquidity,
		SecondPosition:          secondPosition,
	}, nil
}

// ComputeSqrtPriceStepBps is the sqrt-price step that exhausts a market-cap fee schedule.
func ComputeSqrtPriceStepBps(priceMultiple float64, numberOfPeriod int) (uint64, error) {
	if priceMultiple <= 1 {
		return 0, errors.New("priceMultiple must be greater than 1")
	}
	if numberOfPeriod <= 0 {
		return 0, errors.New("numberOfPeriod must be greater than 0")
	}
	step := math.Floor((math.Sqrt(priceMultiple) - 1) * float64(shared.MaxBasisPoint) / float64(numberOfPeriod))
	if step <= 0 {
		return 0, errors.New("sqrtPriceStepBps rounds to zero for the given priceMultiple and numberOfPeriod")
	}
	return uint64(step), nil
}
