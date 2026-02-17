package helpers

import (
	"context"
	"fmt"
	"math/big"

	bin "github.com/gagliardetto/binary"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// NativeMint is the wrapped SOL mint.
var NativeMint = solanago.WrappedSol

func GetTokenProgram(flag uint8) solanago.PublicKey {
	if flag == 0 {
		return token.ProgramID
	}
	return solanago.Token2022ProgramID
}

func GetTokenDecimals(ctx context.Context, client *rpc.Client, mint solanago.PublicKey, tokenProgram solanago.PublicKey) (uint8, error) {
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

// GetAllUserPositionNftAccount finds Token2022 accounts with amount==1 and owner filter applied.
func GetAllUserPositionNftAccount(ctx context.Context, client *rpc.Client, user solanago.PublicKey) ([]PositionNftAccount, error) {
	filters := []rpc.RPCFilter{
		{Memcmp: &rpc.RPCFilterMemcmp{Offset: 32, Bytes: solanago.Base58(user.Bytes())}},
		{Memcmp: &rpc.RPCFilterMemcmp{Offset: 64, Bytes: solanago.Base58([]byte{1, 0, 0, 0, 0, 0, 0, 0})}},
	}
	resp, err := client.GetProgramAccountsWithOpts(ctx, solanago.Token2022ProgramID, &rpc.GetProgramAccountsOpts{Filters: filters})
	if err != nil {
		return nil, err
	}
	out := make([]PositionNftAccount, 0, len(resp))
	for _, acc := range resp {
		data := acc.Account.Data.GetBinary()
		dec := bin.NewBinDecoder(data)
		var tokenAcc token.Account
		if err := tokenAcc.UnmarshalWithDecoder(dec); err != nil {
			continue
		}
		out = append(out, PositionNftAccount{PositionNft: tokenAcc.Mint, PositionNftAccount: acc.Pubkey})
	}
	return out, nil
}

// GetAllPositionNftAccountByOwner loads all Token2022 token accounts and returns those with amount==1.
func GetAllPositionNftAccountByOwner(ctx context.Context, client *rpc.Client, user solanago.PublicKey) ([]PositionNftAccount, error) {
	programID := solanago.Token2022ProgramID
	resp, err := client.GetTokenAccountsByOwner(ctx, user, &rpc.GetTokenAccountsConfig{ProgramId: &programID}, &rpc.GetTokenAccountsOpts{})
	if err != nil {
		return nil, err
	}
	out := make([]PositionNftAccount, 0, len(resp.Value))
	for _, acc := range resp.Value {
		dec := bin.NewBinDecoder(acc.Account.Data.GetBinary())
		var tokenAcc token.Account
		if err := tokenAcc.UnmarshalWithDecoder(dec); err != nil {
			continue
		}
		if tokenAcc.Amount == 1 {
			out = append(out, PositionNftAccount{PositionNft: tokenAcc.Mint, PositionNftAccount: acc.Pubkey})
		}
	}
	return out, nil
}

func GetTokenInfo(ctx context.Context, client *rpc.Client, mint solanago.PublicKey) (*TokenInfo, error) {

	out, err := client.GetAccountInfo(ctx, mint)
	if err != nil {
		return nil, err
	}

	if !out.Value.Owner.Equals(solanago.Token2022ProgramID) {
		return nil, nil
	}

	mintAcc := &token.Mint{}
	if err = mintAcc.Decode(out.GetBinary()); err != nil {
		return nil, err
	}

	epochInfo, err := client.GetEpochInfo(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}

	ext, err := parseToken2022Extensions(out.GetBinary())
	if err != nil {
		return nil, err
	}

	if ext.TransferFeeConfig == nil {
		return &TokenInfo{
			Mint:            mint,
			CurrentEpoch:    epochInfo.Epoch,
			Decimals:        mintAcc.Decimals,
			HasTransferFee:  false,
			HasTransferHook: ext.HasTransferHook,
		}, nil
	}

	fee := ext.TransferFeeConfig.FeeForEpoch(epochInfo.Epoch)

	return &TokenInfo{
		Mint:            mint,
		CurrentEpoch:    epochInfo.Epoch,
		Decimals:        mintAcc.Decimals,
		BasisPoints:     fee.FeeBps,
		MaximumFee:      new(big.Int).SetUint64(fee.MaxFee),
		HasTransferFee:  true,
		HasTransferHook: ext.HasTransferHook,
	}, nil
}
