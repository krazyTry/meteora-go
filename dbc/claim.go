package dbc

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"
)

// ClaimPartnerTradingFeeInstruction generates the instruction needed for partners to claim transaction fees.
// The function includes the creation of account ATA.
//
// Example:
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// configState, _ := m.GetConfig(ctx, poolState.Config)
//
// instructions, _ := ClaimPartnerTradingFeeInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	m.feeClaimer.PublicKey(), // partner
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//	claimBase, // true baseTokenMint or false quoteTokenMint
//	maxAmount, // poolState.PartnerBaseFee or poolState.PartnerQuoteFee
//
// )
func ClaimPartnerTradingFeeInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolPartner solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	claimBase bool,
	maxAmount uint64,
) ([]solana.Instruction, error) {
	var maxAmountBase, maxAmountQuote uint64

	if claimBase {
		maxAmountBase = maxAmount
	} else {
		maxAmountQuote = maxAmount
	}

	baseMint := poolState.BaseMint     // baseMint
	quoteMint := configState.QuoteMint // solana.WrappedSol

	baseVault := poolState.BaseVault   // dbc.DeriveTokenVaultPDA(pool, baseMint)
	quoteVault := poolState.QuoteVault // dbc.DeriveTokenVaultPDA(pool, quoteMint)

	tokenBaseProgram := dbc.GetTokenProgram(configState.TokenType)
	tokenQuoteProgram := solana.TokenProgramID

	var instructions []solana.Instruction

	tokenBaseAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, poolPartner, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, poolPartner, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	claimIx, err := dbc.NewClaimTradingFeeInstruction(
		maxAmountBase,
		maxAmountQuote,
		poolAuthority,
		poolState.Config,
		poolAddress,
		tokenBaseAccount,
		tokenQuoteAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		poolPartner,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		dbc.ProgramID,
	)

	if err != nil {
		return nil, err
	}

	instructions = append(instructions, claimIx)

	switch {
	case baseMint.Equals(solana.WrappedSol):
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenBaseAccount,
			poolPartner,
			poolPartner,
			nil,
		).Build()

		instructions = append(instructions, closeWSOLIx)
	case quoteMint.Equals(solana.WrappedSol):
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			poolPartner,
			poolPartner,
			nil,
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

// ClaimPartnerTradingFee claims the trading fee for the partner.
// The function depends on ClaimPartnerTradingFeeInstruction.
// The function is blocking; it will wait for on-chain confirmation before returning.
//
// Example:
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// sig, _ := meteoraDBC.ClaimPartnerTradingFee(
//
//	ctx,
//	wsClient,
//	payer, // wallet, requires signature and ability to pay ATA deposit
//	baseMint, // baseTokenMint address
//	false, // true baseTokenMint or false quoteTokenMint
//	poolState.PartnerQuoteFee, // poolState.PartnerBaseFee or poolState.PartnerQuoteFee
//
// )
func (m *DBC) ClaimPartnerTradingFee(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
	claimBase bool,
	maxAmount uint64,
) (string, error) {

	if maxAmount <= 0 {
		return "", fmt.Errorf("claim amount must be greater than 0")
	}

	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", err
	}

	instructions, err := ClaimPartnerTradingFeeInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.feeClaimer.PublicKey(),
		poolState.Address,
		poolState.VirtualPool,
		configState,
		claimBase,
		maxAmount,
	)
	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			case key.Equals(m.feeClaimer.PublicKey()):
				return &m.feeClaimer.PrivateKey
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

// ClaimCreatorTradingFeeInstruction generates the instruction needed for creators to claim transaction fees.
// The function includes the creation of account ATA.
//
// Example:
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// configState, _ := m.GetConfig(ctx, poolState.Config)
//
// instructions, _ := ClaimPartnerTradingFeeInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	m.poolCreator.PublicKey(), // creator
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//	claimBase, // true baseTokenMint or false quoteTokenMint
//	maxAmount, // poolState.CreatorBaseFee or poolState.CreatorQuoteFee
//
// )
func ClaimCreatorTradingFeeInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	claimBase bool,
	maxAmount uint64,
) ([]solana.Instruction, error) {
	var maxAmountBase, maxAmountQuote uint64

	if claimBase {
		maxAmountBase = maxAmount
	} else {
		maxAmountQuote = maxAmount
	}
	baseMint := poolState.BaseMint
	quoteMint := configState.QuoteMint

	baseVault := poolState.BaseVault
	quoteVault := poolState.QuoteVault

	tokenBaseProgram := dbc.GetTokenProgram(configState.TokenType)
	tokenQuoteProgram := solana.TokenProgramID

	var instructions []solana.Instruction

	tokenBaseAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, poolCreator, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, poolCreator, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	claimIx, err := dbc.NewClaimCreatorTradingFeeInstruction(
		maxAmountBase,
		maxAmountQuote,

		// Accounts:
		poolAuthority,
		poolAddress,
		tokenBaseAccount,
		tokenQuoteAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		poolCreator,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		dbc.ProgramID,
	)

	if err != nil {
		return nil, err
	}

	instructions = append(instructions, claimIx)

	switch {
	case baseMint.Equals(solana.WrappedSol):
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenBaseAccount,
			poolCreator,
			poolCreator,
			nil,
		).Build()

		instructions = append(instructions, closeWSOLIx)
	case quoteMint.Equals(solana.WrappedSol):
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			poolCreator,
			poolCreator,
			nil,
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

// ClaimCreatorTradingFee claims a creator trading fee.
// If your pool's config key has creatorTradingFeePercentage > 0, you can use this function to claim the trading fee for the pool creator.
// The function depends on ClaimCreatorTradingFeeInstruction.
// The function is blocking; it will wait for on-chain confirmation before returning.
//
// Example:
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// sig, _ := meteoraDBC.ClaimCreatorTradingFee(
//
//	ctx,
//	wsClient,
//	payer, // wallet, requires signature and ability to pay ATA deposit
//	baseMint, // baseTokenMint address
//	false, // true baseTokenMint or false quoteTokenMint
//	poolState.PartnerQuoteFee, // poolState.CreatorBaseFee or poolState.CreatorQuoteFee
//
// )
func (m *DBC) ClaimCreatorTradingFee(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
	claimBase bool,
	maxAmount uint64,
) (string, error) {

	if maxAmount <= 0 {
		return "", fmt.Errorf("claim amount must be greater than 0")
	}

	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", err
	}

	instructions, err := ClaimCreatorTradingFeeInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		poolState.Address,
		poolState.VirtualPool,
		configState,
		claimBase,
		maxAmount,
	)
	if err != nil {
		return "", err
	}

	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		wsClient,
		instructions,
		payer.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(payer.PublicKey()):
				return &payer.PrivateKey
			case key.Equals(m.poolCreator.PublicKey()):
				return &m.poolCreator.PrivateKey
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
