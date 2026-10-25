package helpers

import (
	"context"
	"fmt"
	"math/big"

	bin "github.com/gagliardetto/binary"
	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
	"github.com/solana-foundation/solana-go/v2"
	solanago "github.com/solana-foundation/solana-go/v2"
	"github.com/solana-foundation/solana-go/v2/programs/system"
	"github.com/solana-foundation/solana-go/v2/programs/token"
	"github.com/solana-foundation/solana-go/v2/rpc"
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

func GetTokenProgram(tokenType shared.TokenType) solanago.PublicKey {
	if tokenType == shared.TokenTypeSPL {
		return token.ProgramID
	}
	return solanago.Token2022ProgramID
}

// PrepareSwapAccountsParams are the token accounts a caller-built swap instruction needs.
type PrepareSwapAccountsParams struct {
	Payer                    solanago.PublicKey
	InputOwner               solanago.PublicKey
	OutputOwner              solanago.PublicKey
	UnwrapAuthority          solanago.PublicKey
	InputMint                solanago.PublicKey
	OutputMint               solanago.PublicKey
	InputTokenProgram        solanago.PublicKey
	OutputTokenProgram       solanago.PublicKey
	WrapAmount               uint64
	IncludeInstructionSysvar bool
}

// PrepareSwapAccountsResult holds the ATAs, wrap and unwrap instructions, and the instructions sysvar when requested.
type PrepareSwapAccountsResult struct {
	InputTokenAccount  solanago.PublicKey
	OutputTokenAccount solanago.PublicKey
	PreInstructions    []solanago.Instruction
	PostInstructions   []solanago.Instruction
	RemainingAccounts  []*solanago.AccountMeta
}

// PrepareSwapAccounts creates the input and output token accounts, wraps and unwraps SOL, and optionally adds the instructions sysvar.
func PrepareSwapAccounts(ctx context.Context, client *rpc.Client, params PrepareSwapAccountsParams) (PrepareSwapAccountsResult, error) {
	input, createIn, err := GetOrCreateATAInstruction(ctx, client, params.InputMint, params.InputOwner, params.Payer, params.InputTokenProgram)
	if err != nil {
		return PrepareSwapAccountsResult{}, err
	}
	output, createOut, err := GetOrCreateATAInstruction(ctx, client, params.OutputMint, params.OutputOwner, params.Payer, params.OutputTokenProgram)
	if err != nil {
		return PrepareSwapAccountsResult{}, err
	}
	pre := make([]solanago.Instruction, 0, 2)
	if createIn != nil {
		pre = append(pre, createIn)
	}
	if createOut != nil {
		pre = append(pre, createOut)
	}
	if IsNativeSol(params.InputMint) {
		wrap, err := WrapSOLInstruction(params.InputOwner, input, params.WrapAmount)
		if err != nil {
			return PrepareSwapAccountsResult{}, err
		}
		pre = append(pre, wrap...)
	}
	post := make([]solanago.Instruction, 0, 1)
	if IsNativeSol(params.InputMint) || IsNativeSol(params.OutputMint) {
		unwrap, err := UnwrapSOLInstruction(params.UnwrapAuthority, params.UnwrapAuthority, true)
		if err != nil {
			return PrepareSwapAccountsResult{}, err
		}
		if unwrap != nil {
			post = append(post, unwrap)
		}
	}
	var remaining []*solanago.AccountMeta
	if params.IncludeInstructionSysvar {
		remaining = []*solanago.AccountMeta{solanago.NewAccountMeta(solanago.SysVarInstructionsPubkey, false, false)}
	}
	return PrepareSwapAccountsResult{
		InputTokenAccount:  input,
		OutputTokenAccount: output,
		PreInstructions:    pre,
		PostInstructions:   post,
		RemainingAccounts:  remaining,
	}, nil
}

// GetOrCreateATAs returns the ATAs for several mint and owner pairs, plus create instructions for the missing ones.
func GetOrCreateATAs(ctx context.Context, client *rpc.Client, payer solanago.PublicKey, accounts []struct {
	Mint         solanago.PublicKey
	Owner        solanago.PublicKey
	TokenProgram solanago.PublicKey
}) ([]solanago.PublicKey, []solanago.Instruction, error) {
	addresses := make([]solanago.PublicKey, 0, len(accounts))
	instructions := make([]solanago.Instruction, 0)
	for _, account := range accounts {
		ata, ix, err := GetOrCreateATAInstruction(ctx, client, account.Mint, account.Owner, payer, account.TokenProgram)
		if err != nil {
			return nil, nil, err
		}
		addresses = append(addresses, ata)
		if ix != nil {
			instructions = append(instructions, ix)
		}
	}
	return addresses, instructions, nil
}

// PrepareSwapAmountParam converts a UI amount to lamports using the mint's on-chain decimals.
func PrepareSwapAmountParam(ctx context.Context, client *rpc.Client, amount string, mint solanago.PublicKey) (*big.Int, error) {
	decimals, err := GetTokenDecimals(ctx, client, mint)
	if err != nil {
		return nil, err
	}
	return ConvertToLamports(amount, int32(decimals))
}

func GetTokenType(ctx context.Context, client *rpc.Client, tokenMint solanago.PublicKey) (shared.TokenType, error) {
	acc, err := client.GetAccountInfo(ctx, tokenMint)
	if err != nil {
		return shared.TokenTypeSPL, err
	}
	if acc == nil || acc.Value == nil {
		return shared.TokenTypeSPL, fmt.Errorf("mint not found")
	}
	owner := acc.Value.Owner
	if owner.Equals(token.ProgramID) {
		return shared.TokenTypeSPL, nil
	}
	return shared.TokenTypeToken2022, nil
}
