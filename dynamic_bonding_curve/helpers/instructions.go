package helpers

import (
	dammv1 "github.com/krazyTry/meteora-go/gen/damm_v1"
	dynamicvault "github.com/krazyTry/meteora-go/gen/dynamic_vault"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
)

// CreateInitializePermissionlessDynamicVaultIx builds the initialize vault instruction with derived PDAs.
func CreateInitializePermissionlessDynamicVaultIx(mint, payer solanago.PublicKey) (vaultKey, tokenVaultKey, lpMintKey solanago.PublicKey, instruction solanago.Instruction, err error) {
	vaultKey = DeriveVaultAddress(mint, BaseAddress)
	tokenVaultKey = DeriveTokenVaultKey(vaultKey)
	lpMintKey = DeriveVaultLpMintAddress(vaultKey)

	instruction, err = dynamicvault.NewInitializeInstruction(
		vaultKey,
		payer,
		tokenVaultKey,
		mint,
		lpMintKey,
		solanago.SysVarRentPubkey,
		token.ProgramID,
		system.ProgramID,
	)
	return
}

// CreateLockEscrowIx creates a DAMM V1 lock escrow instruction.
func CreateLockEscrowIx(payer, pool, lpMint, escrowOwner, lockEscrowKey solanago.PublicKey) (solanago.Instruction, error) {
	return dammv1.NewCreateLockEscrowInstruction(
		pool,
		lpMint,
		escrowOwner,
		lockEscrowKey,
		payer,
		system.ProgramID,
	)
}
