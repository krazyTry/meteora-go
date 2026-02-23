package helpers

import (
	"bytes"

	solanago "github.com/gagliardetto/solana-go"
)

var seed = struct {
	PoolAuthority           []byte
	EventAuthority          []byte
	Pool                    []byte
	TokenVault              []byte
	Metadata                []byte
	PartnerMetadata         []byte
	ClaimFeeOperator        []byte
	DammV1MigrationMetadata []byte
	DammV2MigrationMetadata []byte
	LpMint                  []byte
	Fee                     []byte
	Position                []byte
	PositionNftAccount      []byte
	LockEscrow              []byte
	VirtualPoolMetadata     []byte
	Escrow                  []byte
	BaseLocker              []byte
	Vault                   []byte
}{
	PoolAuthority:           []byte("pool_authority"),
	EventAuthority:          []byte("__event_authority"),
	Pool:                    []byte("pool"),
	TokenVault:              []byte("token_vault"),
	Metadata:                []byte("metadata"),
	PartnerMetadata:         []byte("partner_metadata"),
	ClaimFeeOperator:        []byte("cf_operator"),
	DammV1MigrationMetadata: []byte("meteora"),
	DammV2MigrationMetadata: []byte("damm_v2"),
	LpMint:                  []byte("lp_mint"),
	Fee:                     []byte("fee"),
	Position:                []byte("position"),
	PositionNftAccount:      []byte("position_nft_account"),
	LockEscrow:              []byte("lock_escrow"),
	VirtualPoolMetadata:     []byte("virtual_pool_metadata"),
	Escrow:                  []byte("escrow"),
	BaseLocker:              []byte("base_locker"),
	Vault:                   []byte("vault"),
}

func DeriveDbcEventAuthority() solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.EventAuthority}, DynamicBondingCurveProgramID)
	return pub
}

func DeriveDammV1EventAuthority() solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.EventAuthority}, DammV1ProgramID)
	return pub
}

func DeriveDammV2EventAuthority() solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.EventAuthority}, DammV2ProgramID)
	return pub
}

func DeriveLockerEventAuthority() solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.EventAuthority}, LockerProgramID)
	return pub
}

func DeriveDbcPoolAuthority() solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.PoolAuthority}, DynamicBondingCurveProgramID)
	return pub
}

func DeriveDammV1PoolAuthority() solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.PoolAuthority}, DammV1ProgramID)
	return pub
}

func DeriveDammV2PoolAuthority() solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.PoolAuthority}, DammV2ProgramID)
	return pub
}

func DeriveDbcPoolAddress(quoteMint, baseMint, config solanago.PublicKey) solanago.PublicKey {
	isQuoteBigger := bytes.Compare(quoteMint.Bytes(), baseMint.Bytes()) > 0
	var first, second solanago.PublicKey
	if isQuoteBigger {
		first = quoteMint
		second = baseMint
	} else {
		first = baseMint
		second = quoteMint
	}
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.Pool, config.Bytes(), first.Bytes(), second.Bytes()}, DynamicBondingCurveProgramID)
	return pub
}

func DeriveDammV1PoolAddress(config, tokenAMint, tokenBMint solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{
		GetFirstKey(tokenAMint, tokenBMint),
		GetSecondKey(tokenAMint, tokenBMint),
		config.Bytes(),
	}, DammV1ProgramID)
	return pub
}

func DeriveDammV2PoolAddress(config, tokenAMint, tokenBMint solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{
		seed.Pool,
		config.Bytes(),
		GetFirstKey(tokenAMint, tokenBMint),
		GetSecondKey(tokenAMint, tokenBMint),
	}, DammV2ProgramID)
	return pub
}

func DeriveDbcTokenVaultAddress(pool solanago.PublicKey, mint solanago.PublicKey) solanago.PublicKey {
	// Seed order matches on-chain: ["token_vault", mint, pool]
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.TokenVault, mint.Bytes(), pool.Bytes()}, DynamicBondingCurveProgramID)
	return pub
}

func DeriveMintMetadata(mint solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.Metadata, MetaplexProgramID.Bytes(), mint.Bytes()}, MetaplexProgramID)
	return pub
}

func DerivePartnerMetadata(feeClaimer solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.PartnerMetadata, feeClaimer.Bytes()}, DynamicBondingCurveProgramID)
	return pub
}

func DeriveClaimFeeOperatorAddress(config solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.ClaimFeeOperator, config.Bytes()}, DynamicBondingCurveProgramID)
	return pub
}

func DeriveDammV1MigrationMetadataAddress(pool solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.DammV1MigrationMetadata, pool.Bytes()}, DynamicBondingCurveProgramID)
	return pub
}

func DeriveDammV2MigrationMetadataAddress(pool solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.DammV2MigrationMetadata, pool.Bytes()}, DynamicBondingCurveProgramID)
	return pub
}

func DeriveDammV1LpMintAddress(pool solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.LpMint, pool.Bytes()}, DammV1ProgramID)
	return pub
}

// DeriveDammV1VaultLPAddress derives the DAMM V1 vault LP address.
func DeriveDammV1VaultLPAddress(vault, pool solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{vault.Bytes(), pool.Bytes()}, DammV1ProgramID)
	return pub
}

// DeriveDammV1ProtocolFeeAddress derives the DAMM V1 protocol fee address.
func DeriveDammV1ProtocolFeeAddress(mint, pool solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.Fee, mint.Bytes(), pool.Bytes()}, DammV1ProgramID)
	return pub
}

// DeriveDammV2TokenVaultAddress derives the DAMM V2 token vault address.
func DeriveDammV2TokenVaultAddress(pool, mint solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.TokenVault, mint.Bytes(), pool.Bytes()}, DammV2ProgramID)
	return pub
}

// DerivePositionAddress derives the DAMM V2 position PDA from the position NFT mint.
func DerivePositionAddress(positionNftMint solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.Position, positionNftMint.Bytes()}, DammV2ProgramID)
	return pub
}

// DerivePositionNftAccount derives the DAMM V2 position NFT account PDA.
func DerivePositionNftAccount(positionNftMint solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.PositionNftAccount, positionNftMint.Bytes()}, DammV2ProgramID)
	return pub
}

// DeriveDammV2PositionVestingAccount derives the DAMM V2 position vesting account PDA.
func DeriveDammV2PositionVestingAccount(position solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{[]byte("position_vesting"), position.Bytes()}, DynamicBondingCurveProgramID)
	return pub
}

// DeriveDammV1LockEscrowAddress derives the DAMM V1 lock escrow PDA.
func DeriveDammV1LockEscrowAddress(pool, owner solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.LockEscrow, pool.Bytes(), owner.Bytes()}, DammV1ProgramID)
	return pub
}

func DeriveVirtualPoolMetadata(pool solanago.PublicKey, index uint8) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.VirtualPoolMetadata, pool.Bytes(), []byte{index}}, DynamicBondingCurveProgramID)
	return pub
}

// DeriveEscrow derives the locker escrow PDA.
func DeriveEscrow(base solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.Escrow, base.Bytes()}, LockerProgramID)
	return pub
}

// DeriveBaseKeyForLocker derives the base key for the locker.
func DeriveBaseKeyForLocker(virtualPool solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.BaseLocker, virtualPool.Bytes()}, DynamicBondingCurveProgramID)
	return pub
}

// DeriveBaseLockerAddress derives the locker base PDA (legacy helper).
func DeriveBaseLockerAddress(pool solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.BaseLocker, pool.Bytes()}, LockerProgramID)
	return pub
}

func DeriveVaultAddress(mint solanago.PublicKey, baseAddress solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.Vault, baseAddress.Bytes(), mint.Bytes()}, VaultProgramID)
	return pub
}

func DeriveTokenVaultKey(vault solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.TokenVault, vault.Bytes()}, VaultProgramID)
	return pub
}

func DeriveVaultLpMintAddress(vault solanago.PublicKey) solanago.PublicKey {
	pub, _, _ := solanago.FindProgramAddress([][]byte{seed.LpMint, vault.Bytes()}, VaultProgramID)
	return pub
}
