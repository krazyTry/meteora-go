package helpers

import (
	"math/big"

	binary "github.com/gagliardetto/binary"

	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
)

func EncodeFeeTimeSchedulerParams(cliffFeeNumerator *bigInt, numberOfPeriod uint16, periodFrequency *bigInt, reductionFactor *bigInt, baseFeeMode uint8) ([]byte, error) {
	params := dammv2gen.BorshFeeTimeScheduler{
		CliffFeeNumerator: toU64(cliffFeeNumerator),
		NumberOfPeriod:    numberOfPeriod,
		PeriodFrequency:   toU64(periodFrequency),
		ReductionFactor:   toU64(reductionFactor),
		BaseFeeMode:       baseFeeMode,
		Padding:           FeePadding,
	}
	return params.Marshal()
}

func DecodeFeeTimeSchedulerParams(data []byte) (dammv2gen.BorshFeeTimeScheduler, error) {
	var out dammv2gen.BorshFeeTimeScheduler
	if err := out.UnmarshalWithDecoder(binary.NewBorshDecoder(data)); err != nil {
		return dammv2gen.BorshFeeTimeScheduler{}, err
	}
	return out, nil
}

func DecodePodAlignedFeeTimeScheduler(data []byte) (dammv2gen.PodAlignedFeeTimeScheduler, error) {
	var out dammv2gen.PodAlignedFeeTimeScheduler
	if err := out.UnmarshalWithDecoder(binary.NewBorshDecoder(data)); err != nil {
		return dammv2gen.PodAlignedFeeTimeScheduler{}, err
	}
	return out, nil
}

func EncodeFeeMarketCapSchedulerParams(cliffFeeNumerator *bigInt, numberOfPeriod uint16, sqrtPriceStepBps uint16, schedulerExpirationDuration uint32, reductionFactor *bigInt, baseFeeMode uint8) ([]byte, error) {
	params := dammv2gen.BorshFeeMarketCapScheduler{
		CliffFeeNumerator:           toU64(cliffFeeNumerator),
		NumberOfPeriod:              numberOfPeriod,
		SqrtPriceStepBps:            uint32(sqrtPriceStepBps),
		SchedulerExpirationDuration: schedulerExpirationDuration,
		ReductionFactor:             toU64(reductionFactor),
		BaseFeeMode:                 baseFeeMode,
		Padding:                     FeePadding,
	}
	return params.Marshal()
}

func DecodeFeeMarketCapSchedulerParams(data []byte) (dammv2gen.BorshFeeMarketCapScheduler, error) {
	var out dammv2gen.BorshFeeMarketCapScheduler
	if err := out.UnmarshalWithDecoder(binary.NewBorshDecoder(data)); err != nil {
		return dammv2gen.BorshFeeMarketCapScheduler{}, err
	}
	return out, nil
}

func DecodePodAlignedFeeMarketCapScheduler(data []byte) (dammv2gen.PodAlignedFeeMarketCapScheduler, error) {
	var out dammv2gen.PodAlignedFeeMarketCapScheduler
	if err := out.UnmarshalWithDecoder(binary.NewBorshDecoder(data)); err != nil {
		return dammv2gen.PodAlignedFeeMarketCapScheduler{}, err
	}
	return out, nil
}

func EncodeFeeRateLimiterParams(cliffFeeNumerator *bigInt, feeIncrementBps uint16, maxLimiterDuration uint32, maxFeeBps uint16, referenceAmount *bigInt) ([]byte, error) {
	params := dammv2gen.BorshFeeRateLimiter{
		CliffFeeNumerator:  toU64(cliffFeeNumerator),
		FeeIncrementBps:    feeIncrementBps,
		MaxLimiterDuration: maxLimiterDuration,
		MaxFeeBps:          uint32(maxFeeBps),
		ReferenceAmount:    toU64(referenceAmount),
		BaseFeeMode:        BaseFeeModeRateLimiter,
		Padding:            FeePadding,
	}
	return params.Marshal()
}

func DecodeFeeRateLimiterParams(data []byte) (dammv2gen.BorshFeeRateLimiter, error) {
	var out dammv2gen.BorshFeeRateLimiter
	if err := out.UnmarshalWithDecoder(binary.NewBorshDecoder(data)); err != nil {
		return dammv2gen.BorshFeeRateLimiter{}, err
	}
	return out, nil
}

func DecodePodAlignedFeeRateLimiter(data []byte) (dammv2gen.PodAlignedFeeRateLimiter, error) {
	var out dammv2gen.PodAlignedFeeRateLimiter
	if err := out.UnmarshalWithDecoder(binary.NewBorshDecoder(data)); err != nil {
		return dammv2gen.PodAlignedFeeRateLimiter{}, err
	}
	return out, nil
}

// bigInt is a local alias for readability.
type bigInt = big.Int

func toU64(v *bigInt) uint64 {
	if v == nil {
		return 0
	}
	return v.Uint64()
}
