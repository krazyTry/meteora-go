package dynamic_bonding_curve

import (
	solanago "github.com/gagliardetto/solana-go"
	dammv1gen "github.com/krazyTry/meteora-go/gen/damm_v1"
	dammv2gen "github.com/krazyTry/meteora-go/gen/damm_v2"
	dbcgen "github.com/krazyTry/meteora-go/gen/dynamic_bonding_curve"
	vaultgen "github.com/krazyTry/meteora-go/gen/dynamic_vault"
)

var (
	DynamicBondingCurveProgramID = dbcgen.ProgramID
	MetaplexProgramID            = solanago.MustPublicKeyFromBase58("metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s")
	DammV1ProgramID              = dammv1gen.ProgramID
	DammV2ProgramID              = dammv2gen.ProgramID
	VaultProgramID               = vaultgen.ProgramID
	LockerProgramID              = solanago.MustPublicKeyFromBase58("LocpQgucEQHbqNABEYvBvwoxCPsSbG91A1QaQhQQqjn")
	BaseAddress                  = solanago.MustPublicKeyFromBase58("HWzXGcGHy4tcpYfaRDCyLNzXqBTv3E6BttpCH2vJxArv")
)
