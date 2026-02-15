package helpers

import (
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
	dammv1gen "github.com/krazyTry/meteora-go/gen/damm_v1"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
	dbcgen "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"
	vaultgen "github.com/krazyTry/meteora-go/gen/dynamic_vault"
)

const (
	AccountKeyClaimFeeOperator             = "ClaimFeeOperator"
	AccountKeyConfig                       = "Config"
	AccountKeyLockEscrow                   = "LockEscrow"
	AccountKeyMeteoraDammMigrationMetadata = "MeteoraDammMigrationMetadata"
	AccountKeyMeteoraDammV2Metadata        = "MeteoraDammV2Metadata"
	AccountKeyPartnerMetadata              = "PartnerMetadata"
	AccountKeyPoolConfig                   = "PoolConfig"
	AccountKeyVirtualPool                  = "VirtualPool"
	AccountKeyVirtualPoolMetadata          = "VirtualPoolMetadata"

	MaxCurvePoint = 16

	Offset     = 64
	Resolution = 64

	FeeDenominator = 1_000_000_000
	MaxBasisPoint  = 10_000

	U16Max = 65_535
	U24Max = 16_777_215

	MinFeeBps = 25
	MaxFeeBps = 9900

	MinFeeNumerator = 2_500_000
	MaxFeeNumerator = 990_000_000

	MaxRateLimiterDurationInSeconds = 43_200
	MaxRateLimiterDurationInSlots   = 108_000

	DynamicFeeFilterPeriodDefault    = 10
	DynamicFeeDecayPeriodDefault     = 120
	DynamicFeeReductionFactorDefault = 5000
	BinStepBpsDefault                = 1
	MaxPriceChangePercentageDefault  = 20

	ProtocolFeePercent = 20
	HostFeePercent     = 20

	SwapBufferPercentage = 25

	MaxMigrationFeePercentage        = 99
	MaxCreatorMigrationFeePercentage = 100

	MinLockedLiquidityBps    = 1000
	SecondsPerDay            = 86400
	MaxLockDurationInSeconds = 63_072_000

	ProtocolPoolCreationFeePercent = 10
	MinPoolCreationFee             = 1_000_000
	MaxPoolCreationFee             = 100_000_000_000

	MinMigratedPoolFeeBps = 10
	MaxMigratedPoolFeeBps = 1000
)

var (
	OneQ64 = new(big.Int).Lsh(big.NewInt(1), Resolution)

	U64Max  = new(big.Int).SetUint64(^uint64(0))
	U128Max = bigIntFromString("340282366920938463463374607431768211455")

	MinSqrtPrice = bigIntFromString("4295048016")
	MaxSqrtPrice = bigIntFromString("79226673521066979257578248091")

	DynamicFeeScalingFactor  = bigIntFromString("100000000000")
	DynamicFeeRoundingOffset = bigIntFromString("99999999999")

	BinStepBpsU128Default = bigIntFromString("1844674407370955")

	DynamicBondingCurveProgramID = dbcgen.ProgramID
	MetaplexProgramID            = solanago.MustPublicKeyFromBase58("metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s")
	DammV1ProgramID              = dammv1gen.ProgramID
	DammV2ProgramID              = dammv2gen.ProgramID
	VaultProgramID               = vaultgen.ProgramID
	LockerProgramID              = solanago.MustPublicKeyFromBase58("LocpQgucEQHbqNABEYvBvwoxCPsSbG91A1QaQhQQqjn")
	BaseAddress                  = solanago.MustPublicKeyFromBase58("HWzXGcGHy4tcpYfaRDCyLNzXqBTv3E6BttpCH2vJxArv")

	DammV1MigrationFeeAddress = []solanago.PublicKey{
		solanago.MustPublicKeyFromBase58("8f848CEy8eY6PhJ3VcemtBDzPPSD4Vq7aJczLZ3o8MmX"),
		solanago.MustPublicKeyFromBase58("HBxB8Lf14Yj8pqeJ8C4qDb5ryHL7xwpuykz31BLNYr7S"),
		solanago.MustPublicKeyFromBase58("7v5vBdUQHTNeqk1HnduiXcgbvCyVEZ612HLmYkQoAkik"),
		solanago.MustPublicKeyFromBase58("EkvP7d5yKxovj884d2DwmBQbrHUWRLGK6bympzrkXGja"),
		solanago.MustPublicKeyFromBase58("9EZYAJrcqNWNQzP2trzZesP7XKMHA1jEomHzbRsdX8R2"),
		solanago.MustPublicKeyFromBase58("8cdKo87jZU2R12KY1BUjjRPwyjgdNjLGqSGQyrDshhud"),
	}

	DammV2MigrationFeeAddress = []solanago.PublicKey{
		solanago.MustPublicKeyFromBase58("7F6dnUcRuyM2TwR8myT1dYypFXpPSxqwKNSFNkxyNESd"),
		solanago.MustPublicKeyFromBase58("2nHK1kju6XjphBLbNxpM5XRGFj7p9U8vvNzyZiha1z6k"),
		solanago.MustPublicKeyFromBase58("Hv8Lmzmnju6m7kcokVKvwqz7QPmdX9XfKjJsXz8RXcjp"),
		solanago.MustPublicKeyFromBase58("2c4cYd4reUYVRAB9kUUkrq55VPyy2FNQ3FDL4o12JXmq"),
		solanago.MustPublicKeyFromBase58("AkmQWebAwFvWk55wBoCr5D62C6VVDTzi84NJuD9H7cFD"),
		solanago.MustPublicKeyFromBase58("DbCRBj8McvPYHJG1ukj8RE15h2dCNUdTAESG49XpQ44u"),
		solanago.MustPublicKeyFromBase58("A8gMrEPJkacWkcb3DGwtJwTe16HktSEfvwtuDh2MCtck"),
	}

	DefaultLiquidityVestingInfoParams = LiquidityVestingInfoParams{
		LiquidityVestingInfoParameters: LiquidityVestingInfoParameters{
			VestingPercentage:              0,
			BpsPerPeriod:                   0,
			NumberOfPeriods:                0,
			CliffDurationFromMigrationTime: 0,
			Frequency:                      0,
		},
		TotalDuration: 0,
	}

	DefaultMigratedPoolMarketCapFeeSchedulerParams = MigratedPoolMarketCapFeeSchedulerParameters{
		NumberOfPeriod:              0,
		SqrtPriceStepBps:            0,
		SchedulerExpirationDuration: 0,
		ReductionFactor:             0,
	}
)

func bigIntFromString(v string) *big.Int {
	out, ok := new(big.Int).SetString(v, 10)
	if !ok {
		panic("invalid big integer literal")
	}
	return out
}

func GetDammV2Config(migrationFeeOption MigrationFeeOption) solanago.PublicKey {
	switch migrationFeeOption {
	case MigrationFeeOptionFixedBps25:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionFixedBps30:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionFixedBps100:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionFixedBps200:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionFixedBps400:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionFixedBps600:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionCustomizable:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	}
	return solanago.PublicKey{}
}

func GetDammV1Config(migrationFeeOption MigrationFeeOption) solanago.PublicKey {
	switch migrationFeeOption {
	case MigrationFeeOptionFixedBps25:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionFixedBps30:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionFixedBps100:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionFixedBps200:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionFixedBps400:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	case MigrationFeeOptionFixedBps600:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	}
	return solanago.PublicKey{}
}
