package helpers

import (
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
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
)

var (
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

	DefaultLiquidityVestingInfoParams = shared.LiquidityVestingInfoParams{
		LiquidityVestingInfoParameters: shared.LiquidityVestingInfoParameters{
			VestingPercentage:              0,
			BpsPerPeriod:                   0,
			NumberOfPeriods:                0,
			CliffDurationFromMigrationTime: 0,
			Frequency:                      0,
		},
		TotalDuration: 0,
	}

	DefaultMigratedPoolMarketCapFeeSchedulerParams = shared.MigratedPoolMarketCapFeeSchedulerParameters{
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

func GetDammV2Config(migrationFeeOption shared.MigrationFeeOption) solanago.PublicKey {
	switch migrationFeeOption {
	case shared.MigrationFeeOptionFixedBps25:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionFixedBps30:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionFixedBps100:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionFixedBps200:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionFixedBps400:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionFixedBps600:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionCustomizable:
		return DammV2MigrationFeeAddress[migrationFeeOption]
	}
	return solanago.PublicKey{}
}

func GetDammV1Config(migrationFeeOption shared.MigrationFeeOption) solanago.PublicKey {
	switch migrationFeeOption {
	case shared.MigrationFeeOptionFixedBps25:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionFixedBps30:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionFixedBps100:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionFixedBps200:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionFixedBps400:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	case shared.MigrationFeeOptionFixedBps600:
		return DammV1MigrationFeeAddress[migrationFeeOption]
	}
	return solanago.PublicKey{}
}
