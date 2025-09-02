package dbc

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func dbcClaimTradingFee(m *DBC,
	// Params:
	maxAmountBase uint64,
	maxAmountQuote uint64,

	// Accounts:
	config solana.PublicKey,
	dbcPool solana.PublicKey,
	feeClaimer solana.PublicKey,
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseVault solana.PublicKey,
	quoteVault solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority

	program := dbc.ProgramID

	return dbc.NewClaimTradingFeeInstruction(
		maxAmountBase,
		maxAmountQuote,
		poolAuthority,
		config,
		dbcPool,
		tokenBaseAccount,
		tokenQuoteAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		feeClaimer,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) ClaimPartnerTradingFeeInstruction(ctx context.Context,
	payer *solana.Wallet,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
	claimBaseForQuote bool,
	maxAmount uint64,
) ([]solana.Instruction, error) {
	var maxAmountBase, maxAmountQuote uint64

	if claimBaseForQuote {
		maxAmountBase = maxAmount
	} else {
		maxAmountQuote = maxAmount
	}

	baseMint := virtualPool.BaseMint // baseMint
	quoteMint := config.QuoteMint    // solana.WrappedSol

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	if err != nil {
		return nil, err
	}

	baseVault := virtualPool.BaseVault   // dbc.DeriveTokenVaultPDA(pool, baseMint)
	quoteVault := virtualPool.QuoteVault // dbc.DeriveTokenVaultPDA(pool, quoteMint)

	tokenBaseProgram := dbc.GetTokenProgram(config.TokenType)
	tokenQuoteProgram := solana.TokenProgramID

	var instructions []solana.Instruction

	tokenBaseAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, m.feeClaimer.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, m.feeClaimer.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	claimIx, err := dbcClaimTradingFee(m,
		maxAmountBase,
		maxAmountQuote,
		virtualPool.Config,
		pool,
		m.feeClaimer.PublicKey(),
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		tokenBaseAccount,
		tokenQuoteAccount,
		tokenBaseProgram,
		tokenQuoteProgram,
	)
	if err != nil {
		return nil, err
	}

	instructions = append(instructions, claimIx)

	switch {
	case baseMint.Equals(solana.WrappedSol):
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenBaseAccount,
			m.feeClaimer.PublicKey(),
			m.feeClaimer.PublicKey(),
			[]solana.PublicKey{},
		).Build()

		instructions = append(instructions, closeWSOLIx)
	case quoteMint.Equals(solana.WrappedSol):
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			m.feeClaimer.PublicKey(),
			m.feeClaimer.PublicKey(),
			[]solana.PublicKey{},
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

func (m *DBC) ClaimPartnerTradingFee(ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
	claimBaseForQuote bool,
	maxAmount uint64,
) (string, error) {

	if maxAmount <= 0 {
		return "", fmt.Errorf("claim amount must be greater than 0")
	}

	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", err
	}

	instructions, err := m.ClaimPartnerTradingFeeInstruction(ctx, payer, virtualPool, config, claimBaseForQuote, maxAmount)
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
		case key.Equals(m.feeClaimer.PublicKey()):
			return &m.feeClaimer.PrivateKey
		default:
			return nil
		}
	}); err != nil {
		return "", err
	}

	if m.bSimulate {
		if _, err = m.rpcClient.SimulateTransactionWithOpts(
			ctx,
			tx,
			&rpc.SimulateTransactionOpts{
				SigVerify:  false,
				Commitment: rpc.CommitmentFinalized,
			}); err != nil {
			return "", err
		}
		return "-", nil
	}
	sig, err := m.rpcClient.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return "", err
	}

	if _, err = sendandconfirmtransaction.WaitForConfirmation(ctx, m.wsClient, sig, nil); err != nil {
		return "", err
	}
	return sig.String(), nil
}
func claimCreatorTradingFee(m *DBC,
	// Params:
	maxAmountBase uint64,
	maxAmountQuote uint64,

	// Accounts:
	dbcPool solana.PublicKey,
	creator solana.PublicKey,
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	baseVault solana.PublicKey,
	quoteVault solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority

	program := dbc.ProgramID

	return dbc.NewClaimCreatorTradingFeeInstruction(
		maxAmountBase,
		maxAmountQuote,

		// Accounts:
		poolAuthority,
		dbcPool,
		tokenBaseAccount,
		tokenQuoteAccount,
		baseVault,
		quoteVault,
		baseMint,
		quoteMint,
		creator,
		tokenBaseProgram,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) ClaimCreatorTradingFeeInstruction(ctx context.Context,
	payer *solana.Wallet,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
	claimBaseForQuote bool,
	maxAmount uint64,
) ([]solana.Instruction, error) {
	var maxAmountBase, maxAmountQuote uint64

	if claimBaseForQuote {
		maxAmountBase = maxAmount
	} else {
		maxAmountQuote = maxAmount
	}
	baseMint := virtualPool.BaseMint // baseMint
	quoteMint := config.QuoteMint    // solana.WrappedSol

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	// pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	if err != nil {
		return nil, err
	}

	baseVault := virtualPool.BaseVault   // dbc.DeriveTokenVaultPDA(pool, baseMint)
	quoteVault := virtualPool.QuoteVault // dbc.DeriveTokenVaultPDA(pool, quoteMint)

	tokenBaseProgram := dbc.GetTokenProgram(config.TokenType)
	tokenQuoteProgram := solana.TokenProgramID

	var instructions []solana.Instruction

	tokenBaseAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, m.poolCreator.PublicKey(), baseMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, m.poolCreator.PublicKey(), quoteMint, payer.PublicKey(), &instructions)
	if err != nil {
		return nil, err
	}

	claimIx, err := claimCreatorTradingFee(m,
		maxAmountBase,
		maxAmountQuote,
		pool,
		m.poolCreator.PublicKey(),
		baseMint,
		quoteMint,
		baseVault,
		quoteVault,
		tokenBaseAccount,
		tokenQuoteAccount,
		tokenBaseProgram,
		tokenQuoteProgram,
	)
	if err != nil {
		return nil, err
	}

	instructions = append(instructions, claimIx)

	switch {
	case baseMint.Equals(solana.WrappedSol):
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenBaseAccount,
			m.poolCreator.PublicKey(),
			m.poolCreator.PublicKey(),
			[]solana.PublicKey{},
		).Build()

		instructions = append(instructions, closeWSOLIx)
	case quoteMint.Equals(solana.WrappedSol):
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			m.poolCreator.PublicKey(),
			m.poolCreator.PublicKey(),
			[]solana.PublicKey{},
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

func (m *DBC) ClaimCreatorTradingFee(ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
	claimBaseForQuote bool,
	maxAmount uint64,
) (string, error) {

	if maxAmount <= 0 {
		return "", fmt.Errorf("claim amount must be greater than 0")
	}

	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", err
	}

	instructions, err := m.ClaimCreatorTradingFeeInstruction(ctx, payer, virtualPool, config, claimBaseForQuote, maxAmount)
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
		case key.Equals(m.poolCreator.PublicKey()):
			return &m.poolCreator.PrivateKey
		default:
			return nil
		}
	}); err != nil {
		return "", err
	}

	if m.bSimulate {
		if _, err = m.rpcClient.SimulateTransactionWithOpts(
			ctx,
			tx,
			&rpc.SimulateTransactionOpts{
				SigVerify:  false,
				Commitment: rpc.CommitmentFinalized,
			}); err != nil {
			return "", err
		}
		return "-", nil
	}
	sig, err := m.rpcClient.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return "", err
	}
	if _, err = sendandconfirmtransaction.WaitForConfirmation(ctx, m.wsClient, sig, nil); err != nil {
		return "", err
	}
	return sig.String(), nil
}
