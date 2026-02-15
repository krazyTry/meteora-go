package helpers

import (
	"context"
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// NativeMint is the wrapped SOL mint.
var NativeMint = solana.WrappedSol

// GetOrCreateATAInstruction returns the ATA pubkey and an optional create instruction if it doesn't exist.
func GetOrCreateATAInstruction(ctx context.Context, client *rpc.Client, tokenMint, owner, payer solanago.PublicKey, tokenProgram solanago.PublicKey) (solanago.PublicKey, solanago.Instruction, error) {
	ata, err := FindAssociatedTokenAddress(owner, tokenMint, tokenProgram)
	if err != nil {
		return solanago.PublicKey{}, nil, err
	}

	_, err = client.GetAccountInfo(ctx, ata)
	if err == nil {
		return ata, nil, nil
	}

	// create if missing
	ix := CreateAssociatedTokenAccountInstruction(payer, ata, owner, tokenMint, tokenProgram)

	return ata, ix, nil
}

// CreateAssociatedTokenAccountInstruction builds an ATA create instruction that supports custom token programs (SPL/Token2022).
func CreateAssociatedTokenAccountInstruction(payer, ata, owner, mint, tokenProgram solanago.PublicKey) solanago.Instruction {
	accounts := solanago.AccountMetaSlice{
		solanago.NewAccountMeta(payer, true, true),
		solanago.NewAccountMeta(ata, true, false),
		solanago.NewAccountMeta(owner, false, false),
		solanago.NewAccountMeta(mint, false, false),
		solanago.NewAccountMeta(system.ProgramID, false, false),
		solanago.NewAccountMeta(tokenProgram, false, false),
	}
	return solanago.NewInstruction(solanago.SPLAssociatedTokenAccountProgramID, accounts, nil)
}

func UnwrapSOLInstruction(owner, receiver solanago.PublicKey, allowOwnerOffCurve bool) (solanago.Instruction, error) {
	ata, err := FindAssociatedTokenAddress(owner, NativeMint, token.ProgramID)
	if err != nil {
		return nil, err
	}
	return token.NewCloseAccountInstructionBuilder().
		SetAccount(ata).
		SetDestinationAccount(receiver).
		SetOwnerAccount(owner).
		Build(), nil
}

func WrapSOLInstruction(from, to solanago.PublicKey, amount uint64) ([]solanago.Instruction, error) {
	transferIx := system.NewTransferInstructionBuilder().
		SetFundingAccount(from).
		SetRecipientAccount(to).
		SetLamports(amount).
		Build()
	syncIx := token.NewSyncNativeInstructionBuilder().
		SetTokenAccount(to).
		Build()
	return []solanago.Instruction{transferIx, syncIx}, nil
}

func FindAssociatedTokenAddress(wallet, mint, tokenProgram solanago.PublicKey) (solanago.PublicKey, error) {
	ata, _, err := solanago.FindProgramAddress([][]byte{wallet.Bytes(), tokenProgram.Bytes(), mint.Bytes()}, solanago.SPLAssociatedTokenAccountProgramID)
	return ata, err
}

func GetTokenDecimals(ctx context.Context, client *rpc.Client, mint solanago.PublicKey) (uint8, error) {
	acc, err := client.GetAccountInfo(ctx, mint)
	if err != nil {
		return 0, err
	}
	if acc == nil || acc.Value == nil {
		return 0, fmt.Errorf("mint not found")
	}
	dec := bin.NewBinDecoder(acc.Value.Data.GetBinary())
	mintAcc := new(token.Mint)
	if err := mintAcc.UnmarshalWithDecoder(dec); err != nil {
		return 0, err
	}
	return mintAcc.Decimals, nil
}

func GetTokenProgram(tokenType TokenType) solanago.PublicKey {
	if tokenType == TokenTypeSPL {
		return token.ProgramID
	}
	return solanago.Token2022ProgramID
}

func GetTokenType(ctx context.Context, client *rpc.Client, tokenMint solanago.PublicKey) (TokenType, error) {
	acc, err := client.GetAccountInfo(ctx, tokenMint)
	if err != nil {
		return TokenTypeSPL, err
	}
	if acc == nil || acc.Value == nil {
		return TokenTypeSPL, fmt.Errorf("mint not found")
	}
	owner := acc.Value.Owner
	if owner.Equals(token.ProgramID) {
		return TokenTypeSPL, nil
	}
	return TokenTypeToken2022, nil
}
