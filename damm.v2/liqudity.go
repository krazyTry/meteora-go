package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/krazyTry/meteora-go/u128"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
)

func cpAmmAddLiquidity(m *DammV2,
	// delta liquidity
	liquidityDelta *big.Int,
	// maximum token a amount
	tokenBaseAmountThreshold uint64,
	// maximum token b amount
	tokenQuoteAmountThreshold uint64,

	// Accounts:
	owner solana.PublicKey,
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	positionNftAccount solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	tokenBaseMint solana.PublicKey,
	tokenQuoteMint solana.PublicKey,
	tokenBaseVault solana.PublicKey,
	tokenQuoteVault solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {

	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	params := cp_amm.AddLiquidityParameters{
		LiquidityDelta:        u128.GenUint128FromString(liquidityDelta.String()),
		TokenAAmountThreshold: tokenBaseAmountThreshold,
		TokenBAmountThreshold: tokenQuoteAmountThreshold,
	}

	return cp_amm.NewAddLiquidityInstruction(
		params,
		cpammPool,
		position,
		tokenBaseAccount,
		tokenQuoteAccount,
		tokenBaseVault,
		tokenQuoteVault,
		tokenBaseMint,
		tokenQuoteMint,
		positionNftAccount,
		owner,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) AddPositionLiquidityInstruction(ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	virtualPool *Pool,
	userPosition *UserPosition,
	bAddBase bool,
	amountIn *big.Int,
	liquidityDelta *big.Int,
	minOutAmount *big.Int,
) ([]solana.Instruction, error) {
	baseMint := virtualPool.TokenAMint
	quoteMint := virtualPool.TokenBMint

	baseVault := virtualPool.TokenAVault
	quoteVault := virtualPool.TokenBVault

	var instructions []solana.Instruction

	cpammPool, err := m.deriveCpAmmPoolPDA(quoteMint, baseMint)
	if err != nil {
		return nil, err
	}

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	var (
		tokenBaseAmountThreshold  uint64
		tokenQuoteAmountThreshold uint64
	)

	if bAddBase {
		tokenBaseAmountThreshold = cp_amm.U64_MAX
		tokenQuoteAmountThreshold = minOutAmount.Uint64()
	} else {
		tokenBaseAmountThreshold = minOutAmount.Uint64()
		tokenQuoteAmountThreshold = cp_amm.U64_MAX
	}

	if amountIn.Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("amountIn must be greater than 0")
	}

	// wrap SOL by transferring lamports into the WSOL ATA
	wrapSOLIx := system.NewTransferInstruction(
		amountIn.Uint64(),
		owner.PublicKey(),
		quoteTokenAccount,
	).Build()

	// sync the WSOL account to update its balance
	syncNativeIx := token.NewSyncNativeInstruction(
		quoteTokenAccount,
	).Build()

	instructions = append(instructions, wrapSOLIx, syncNativeIx)

	liquidityIx, err := cpAmmAddLiquidity(m,
		liquidityDelta,
		tokenBaseAmountThreshold,
		tokenQuoteAmountThreshold,
		owner.PublicKey(),
		cpammPool,
		userPosition.Position,
		userPosition.PositionNftAccount,
		baseTokenAccount,
		quoteTokenAccount,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		cp_amm.GetTokenProgram(virtualPool.TokenAFlag),
		cp_amm.GetTokenProgram(virtualPool.TokenBFlag),
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, liquidityIx)

	unwrapIx := token.NewCloseAccountInstruction(
		quoteTokenAccount,
		owner.PublicKey(),
		owner.PublicKey(),
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, unwrapIx)

	return instructions, nil
}

func (m *DammV2) AddPositionLiquidity(ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	virtualPool *Pool,
	userPosition *UserPosition,
	bAddBase bool,
	amountIn *big.Int,
	liquidityDelta *big.Int,
	minOutAmount *big.Int,
) (string, error) {
	instructions, err := m.AddPositionLiquidityInstruction(ctx,
		payer,
		owner,
		virtualPool,
		userPosition,
		bAddBase,
		amountIn,
		liquidityDelta,
		minOutAmount,
	)
	if err != nil {
		return "", err
	}

	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(instructions, latestBlockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(payer.PublicKey()):
			return &payer.PrivateKey
		case key.Equals(owner.PublicKey()):
			return &owner.PrivateKey
		default:
			return nil
		}
	}); err != nil {
		return "", err
	}

	if _, err = sendandconfirmtransaction.SendAndConfirmTransaction(ctx, m.rpcClient, m.wsClient, tx); err != nil {
		return "", err
	}
	return tx.Signatures[0].String(), nil
}

func cpAmmRemoveLiquidity(m *DammV2,
	// delta liquidity
	liquidityDelta *big.Int,
	// maximum token a amount
	tokenBaseAmountThreshold uint64,
	// maximum token b amount
	tokenQuoteAmountThreshold uint64,

	// Accounts:
	owner solana.PublicKey,
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	positionNftAccount solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	tokenBaseMint solana.PublicKey,
	tokenQuoteMint solana.PublicKey,
	tokenBaseVault solana.PublicKey,
	tokenQuoteVault solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {
	params := cp_amm.RemoveLiquidityParameters{
		LiquidityDelta:        u128.GenUint128FromString(liquidityDelta.String()),
		TokenAAmountThreshold: tokenBaseAmountThreshold,
		TokenBAmountThreshold: tokenQuoteAmountThreshold,
	}
	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	return cp_amm.NewRemoveLiquidityInstruction(
		params,
		// Accounts:
		poolAuthority,
		cpammPool,
		position,
		tokenBaseAccount,
		tokenQuoteAccount,
		tokenBaseVault,
		tokenQuoteVault,
		tokenBaseMint,
		tokenQuoteMint,
		positionNftAccount,
		owner,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) RemovePositionLiquidityInstruction(ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	virtualPool *Pool,
	userPosition *UserPosition,
	liquidityDelta *big.Int,
	vestings []solana.PublicKey,
) ([]solana.Instruction, error) {
	baseMint := virtualPool.TokenAMint
	quoteMint := virtualPool.TokenBMint

	baseVault := virtualPool.TokenAVault
	quoteVault := virtualPool.TokenBVault

	var instructions []solana.Instruction

	cpammPool, err := m.deriveCpAmmPoolPDA(quoteMint, baseMint)
	if err != nil {
		return nil, err
	}

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	var (
		tokenBaseAmountThreshold  uint64
		tokenQuoteAmountThreshold uint64
	)

	if len(vestings) > 0 {
		var vestingAccounts []*solana.AccountMeta
		for _, v := range vestings {
			vestingAccounts = []*solana.AccountMeta{
				solana.NewAccountMeta(v, false, false),
			}
		}
		refreshVestingIx, err := cpAmmRefreshVesting(cpammPool, userPosition.Position, userPosition.PositionNftAccount, owner.PublicKey(), vestingAccounts)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, refreshVestingIx)
	}

	liquidityIx, err := cpAmmRemoveLiquidity(m,
		liquidityDelta,
		tokenBaseAmountThreshold,
		tokenQuoteAmountThreshold,
		owner.PublicKey(),
		cpammPool,
		userPosition.Position,
		userPosition.PositionNftAccount,
		baseTokenAccount,
		quoteTokenAccount,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		cp_amm.GetTokenProgram(virtualPool.TokenAFlag),
		cp_amm.GetTokenProgram(virtualPool.TokenBFlag),
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, liquidityIx)

	unwrapIx := token.NewCloseAccountInstruction(
		quoteTokenAccount,
		owner.PublicKey(),
		owner.PublicKey(),
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, unwrapIx)
	return instructions, nil
}

func (m *DammV2) RemovePositionLiquidity(ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	virtualPool *Pool,
	userPosition *UserPosition,
	liquidityDelta *big.Int,
	vestings []solana.PublicKey,
) (string, error) {

	instructions, err := m.RemovePositionLiquidityInstruction(ctx,
		payer,
		owner,
		virtualPool,
		userPosition,
		liquidityDelta,
		vestings,
	)
	if err != nil {
		return "", err
	}

	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(instructions, latestBlockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(payer.PublicKey()):
			return &payer.PrivateKey
		// case key.Equals(owner.PublicKey()):
		// 	return &owner.PrivateKey
		default:
			return nil
		}
	}); err != nil {
		return "", err
	}

	if _, err = sendandconfirmtransaction.SendAndConfirmTransaction(ctx, m.rpcClient, m.wsClient, tx); err != nil {
		return "", err
	}
	return tx.Signatures[0].String(), nil
}

func (m *DammV2) GetPositionLiquidity(ctx context.Context, baseMint solana.PublicKey, owner solana.PublicKey) (*big.Int, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return nil, err
	}
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByBaseMint(ctx, virtualPool, owner)
	if err != nil {
		return nil, err
	}
	switch {
	case len(userPositions) == 0:
		return nil, fmt.Errorf("no matching user_position")
	case len(userPositions) == 1:
		userPosition = userPositions[0]
	case len(userPositions) == 2:
		userPosition = userPositions[1]
	}
	return userPosition.PositionState.UnlockedLiquidity.BigInt(), nil
}

func cpAmmRemoveAllLiquidity(m *DammV2,
	// Params:
	tokenBaseAmountThreshold uint64,
	tokenQuoteAmountThreshold uint64,

	// Accounts:
	cpammPool solana.PublicKey,
	position solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	tokenBaseVault solana.PublicKey,
	tokenQuoteVault solana.PublicKey,
	tokenBaseMint solana.PublicKey,
	tokenQuoteMint solana.PublicKey,
	positionNftAccount solana.PublicKey,
	owner solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID

	return cp_amm.NewRemoveAllLiquidityInstruction(
		// Params:
		tokenBaseAmountThreshold,
		tokenQuoteAmountThreshold,

		// Accounts:
		poolAuthority,
		cpammPool,
		position,
		tokenBaseAccount,
		tokenQuoteAccount,
		tokenBaseVault,
		tokenQuoteVault,
		tokenBaseMint,
		tokenQuoteMint,
		positionNftAccount,
		owner,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DammV2) RemoveAllLiquidity(ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	virtualPool *Pool,
	userPosition *UserPosition,
	vestings []solana.PublicKey,
) (string, error) {
	baseMint := virtualPool.TokenAMint
	quoteMint := virtualPool.TokenBMint

	baseVault := virtualPool.TokenAVault
	quoteVault := virtualPool.TokenBVault

	var instructions []solana.Instruction

	cpammPool, err := m.deriveCpAmmPoolPDA(quoteMint, baseMint)
	if err != nil {
		return "", err
	}

	baseTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return "", err
	}

	quoteTokenAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, owner.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return "", err
	}

	var (
		tokenBaseAmountThreshold  uint64
		tokenQuoteAmountThreshold uint64
	)

	if len(vestings) > 0 {
		var vestingAccounts []*solana.AccountMeta
		for _, v := range vestings {
			vestingAccounts = []*solana.AccountMeta{
				solana.NewAccountMeta(v, false, false),
			}
		}
		refreshVestingIx, err := cpAmmRefreshVesting(cpammPool, userPosition.Position, userPosition.PositionNftAccount, owner.PublicKey(), vestingAccounts)
		if err != nil {
			return "", err
		}
		instructions = append(instructions, refreshVestingIx)
	}

	liquidityIx, err := cpAmmRemoveAllLiquidity(m,
		tokenBaseAmountThreshold,
		tokenQuoteAmountThreshold,
		owner.PublicKey(),
		cpammPool,
		userPosition.Position,
		userPosition.PositionNftAccount,
		baseTokenAccount,
		quoteTokenAccount,
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		cp_amm.GetTokenProgram(virtualPool.TokenAFlag),
		cp_amm.GetTokenProgram(virtualPool.TokenBFlag),
	)
	if err != nil {
		return "", err
	}
	instructions = append(instructions, liquidityIx)

	unwrapIx := token.NewCloseAccountInstruction(
		quoteTokenAccount,
		owner.PublicKey(),
		owner.PublicKey(),
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, unwrapIx)

	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction(instructions, latestBlockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(payer.PublicKey()):
			return &payer.PrivateKey
		// case key.Equals(owner.PublicKey()):
		// 	return &owner.PrivateKey
		default:
			return nil
		}
	}); err != nil {
		return "", err
	}

	if _, err = sendandconfirmtransaction.SendAndConfirmTransaction(ctx, m.rpcClient, m.wsClient, tx); err != nil {
		return "", err
	}
	return tx.Signatures[0].String(), nil
}
