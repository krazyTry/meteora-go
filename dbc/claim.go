package dbc

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func ClaimPartnerTradingFeeInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolPartner solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	claimBaseForQuote bool,
	maxAmount uint64,
) ([]solana.Instruction, error) {
	var maxAmountBase, maxAmountQuote uint64

	if claimBaseForQuote {
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

func (m *DBC) ClaimPartnerTradingFee(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
	claimBaseForQuote bool,
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
		claimBaseForQuote,
		maxAmount,
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

func ClaimCreatorTradingFeeInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
	claimBaseForQuote bool,
	maxAmount uint64,
) ([]solana.Instruction, error) {
	var maxAmountBase, maxAmountQuote uint64

	if claimBaseForQuote {
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
			[]solana.PublicKey{},
		).Build()

		instructions = append(instructions, closeWSOLIx)
	case quoteMint.Equals(solana.WrappedSol):
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			poolCreator,
			poolCreator,
			[]solana.PublicKey{},
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

func (m *DBC) ClaimCreatorTradingFee(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
	claimBaseForQuote bool,
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
		claimBaseForQuote,
		maxAmount,
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
