package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func cpAmmSwap(
	m *DammV2,
	cpammPool solana.PublicKey,
	baseMint solana.PublicKey, // tokenAMint solana.PublicKey,
	quoteMint solana.PublicKey, // tokenBMint solana.PublicKey,
	baseVault solana.PublicKey, // tokenAVault solana.PublicKey,
	quoteVault solana.PublicKey, // tokenBVault solana.PublicKey,
	payer solana.PublicKey,
	referralTokenAccount solana.PublicKey,
	inputTokenAccount solana.PublicKey,
	outputTokenAccount solana.PublicKey,
	tokenBaseProgram solana.PublicKey, // tokenAProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey, // tokenBProgram solana.PublicKey,
	amountIn uint64,
	minOut uint64,
) (solana.Instruction, error) {

	return cp_amm.NewSwapInstruction(
		// Params:
		cp_amm.SwapParameters{
			AmountIn:         amountIn,
			MinimumAmountOut: minOut,
		},

		// Accounts:
		poolAuthority,
		cpammPool,
		inputTokenAccount,
		outputTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		payer,
		tokenBaseProgram,
		tokenQuoteProgram,
		referralTokenAccount,
		eventAuthority,
		cp_amm.ProgramID,
	)
}

func SwapInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	owner solana.PublicKey,
	referrer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	swapBaseForQuote bool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) ([]solana.Instruction, error) {
	if amountIn.Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("amountIn must be greater than 0")
	}

	var instructions []solana.Instruction

	inputMint, outputMint, inputMintProgram, outputMintProgram := cp_amm.PrepareSwapParams(swapBaseForQuote, poolState)

	inputTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, inputMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	outputTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, outputMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	baseMint := poolState.TokenAMint
	quoteMint := poolState.TokenBMint

	referralTokenAccount := solana.PublicKey{}

	if !referrer.Equals(solana.PublicKey{}) {
		switch poolState.CollectFeeMode {
		case cp_amm.CollectFeeModeOnlyA:
			referralTokenAccount, err = solanago.PrepareTokenATA(ctx, rpcClient, referrer, baseMint, payer, &instructions)
			if err != nil {
				return nil, err
			}
		case cp_amm.CollectFeeModeOnlyB:
			referralTokenAccount, err = solanago.PrepareTokenATA(ctx, rpcClient, referrer, quoteMint, payer, &instructions)
			if err != nil {
				return nil, err
			}
		}
	}

	switch {
	case inputMint.Equals(solana.WrappedSol):
		// wrap SOL by transferring lamports into the WSOL ATA
		wrapSOLIx := system.NewTransferInstruction(
			amountIn.Uint64(),
			owner,
			inputTokenAccount,
		).Build()
		// sync the WSOL account to update its balance
		syncNativeIx := token.NewSyncNativeInstruction(
			inputTokenAccount,
		).Build()
		instructions = append(instructions, wrapSOLIx, syncNativeIx)
	case outputMint.Equals(solana.WrappedSol):
	}

	baseVault := poolState.TokenAVault
	quoteVault := poolState.TokenBVault

	swapIx, err := cp_amm.NewSwapInstruction(
		// Params:
		cp_amm.SwapParameters{
			AmountIn:         amountIn.Uint64(),
			MinimumAmountOut: minimumAmountOut.Uint64(),
		},

		// Accounts:
		poolAuthority,
		poolAddress,
		inputTokenAccount,
		outputTokenAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		payer,
		inputMintProgram,
		outputMintProgram,
		referralTokenAccount,
		eventAuthority,
		cp_amm.ProgramID,
	)

	if err != nil {
		return nil, err
	}
	instructions = append(instructions, swapIx)

	switch {
	case inputMint.Equals(solana.WrappedSol):
		unwrapIx := token.NewCloseAccountInstruction(
			inputTokenAccount,
			owner,
			owner,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	case outputMint.Equals(solana.WrappedSol):
		unwrapIx := token.NewCloseAccountInstruction(
			outputTokenAccount,
			owner,
			owner,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	if !referrer.Equals(solana.PublicKey{}) && poolState.CollectFeeMode == cp_amm.CollectFeeModeOnlyB {
		unwrapIx := token.NewCloseAccountInstruction(
			referralTokenAccount,
			referrer,
			referrer,
			[]solana.PublicKey{},
		).Build()
		instructions = append(instructions, unwrapIx)
	}

	return instructions, nil
}

func (m *DammV2) Swap(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	swapBaseForQuote bool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) (string, error) {

	instructions, err := SwapInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		owner.PublicKey(),
		func() solana.PublicKey {
			if referrer == nil {
				return solana.PublicKey{}
			}
			return referrer.PublicKey()
		}(),
		poolAddress,
		poolState,
		swapBaseForQuote,
		amountIn,
		minimumAmountOut,
	)

	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", err
	}
	return sig.String(), nil
}

func BuyInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	buyer solana.PublicKey,
	referrer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) ([]solana.Instruction, error) {
	return SwapInstruction(ctx, rpcClient, buyer, buyer, referrer, poolAddress, poolState, false, amountIn, minimumAmountOut)
}

func (m *DammV2) Buy(
	ctx context.Context,
	buyer *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) (string, error) {
	return m.Swap(ctx, buyer, buyer, referrer, poolAddress, poolState, false, amountIn, minimumAmountOut)
}

func SellInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	seller solana.PublicKey,
	referrer solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) ([]solana.Instruction, error) {
	return SwapInstruction(ctx, rpcClient, seller, seller, referrer, poolAddress, poolState, true, amountIn, minimumAmountOut)
}

func (m *DammV2) Sell(
	ctx context.Context,
	seller *solana.Wallet,
	referrer *solana.Wallet,
	poolAddress solana.PublicKey,
	poolState *cp_amm.Pool,
	amountIn *big.Int,
	minimumAmountOut *big.Int,
) (string, error) {
	return m.Swap(ctx, seller, seller, referrer, poolAddress, poolState, true, amountIn, minimumAmountOut)
}
