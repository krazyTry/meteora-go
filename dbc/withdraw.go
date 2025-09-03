package dbc

import (
	"context"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func dbcWithdrawLeftover(
	m *DBC,
	config solana.PublicKey,
	dbcPool solana.PublicKey,
	tokenBaseAccount solana.PublicKey,
	baseVault solana.PublicKey,
	baseMint solana.PublicKey,
	leftoverReceiver solana.PublicKey,
	tokenBaseProgram solana.PublicKey,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority

	program := dbc.ProgramID

	return dbc.NewWithdrawLeftoverInstruction(
		poolAuthority,
		config,
		dbcPool,
		tokenBaseAccount,
		baseVault,
		baseMint,
		leftoverReceiver,
		tokenBaseProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) WithdrawLeftoverInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	leftoverReceiver solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	quoteMint := config.QuoteMint    // solana.WrappedSol
	baseMint := virtualPool.BaseMint // baseMint

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction

	tokenBaseAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, leftoverReceiver, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	baseVault := virtualPool.BaseVault // dbc.DeriveTokenVaultPDA(pool, virtualPool.BaseMint)

	tokenBaseProgram := dbc.GetTokenProgram(config.TokenType)

	withdrawIx, err := dbcWithdrawLeftover(m,
		virtualPool.Config,
		pool,
		tokenBaseAccount,
		baseVault,
		baseMint,
		leftoverReceiver,
		tokenBaseProgram,
	)
	if err != nil {
		return nil, err
	}

	instructions = append(instructions, withdrawIx)
	return instructions, nil
}

func (m *DBC) WithdrawLeftover(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {

	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", err
	}

	instructions, err := m.WithdrawLeftoverInstruction(ctx, payer.PublicKey(), m.leftoverReceiver.PublicKey(), virtualPool, config)
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
			case key.Equals(m.leftoverReceiver.PublicKey()):
				return &m.leftoverReceiver.PrivateKey
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

func dbcWithdrawPartnerSurplus(
	m *DBC,
	config solana.PublicKey,
	dbcPool solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	quoteVault solana.PublicKey,
	quoteMint solana.PublicKey,
	feeClaimer solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {
	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority

	program := dbc.ProgramID

	return dbc.NewPartnerWithdrawSurplusInstruction(
		poolAuthority,
		config,
		dbcPool,
		tokenQuoteAccount,
		quoteVault,
		quoteMint,
		feeClaimer,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) WithdrawPartnerSurplusInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	partner solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	quoteMint := config.QuoteMint    // solana.WrappedSol
	baseMint := virtualPool.BaseMint // baseMint
	quoteVault := virtualPool.QuoteVault

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction
	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, partner, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteProgram := dbc.GetTokenProgram(config.QuoteTokenFlag)

	withdrawIx, err := dbcWithdrawPartnerSurplus(m,
		virtualPool.Config,
		pool,
		tokenQuoteAccount,
		quoteVault,
		quoteMint,
		partner,
		tokenQuoteProgram,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, withdrawIx)

	if quoteMint.Equals(solana.WrappedSol) {
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			partner,
			partner,
			[]solana.PublicKey{},
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

func (m *DBC) WithdrawPartnerSurplus(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", err
	}

	instructions, err := m.WithdrawPartnerSurplusInstruction(ctx, payer.PublicKey(), m.feeClaimer.PublicKey(), virtualPool, config)
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

func dbcWithdrawCreatorSurplus(
	m *DBC,
	config solana.PublicKey,
	dbcPool solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	quoteVault solana.PublicKey,
	quoteMint solana.PublicKey,
	creator solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {
	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority

	program := dbc.ProgramID
	return dbc.NewCreatorWithdrawSurplusInstruction(
		poolAuthority,
		config,
		dbcPool,
		tokenQuoteAccount,
		quoteVault,
		quoteMint,
		creator,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) WithdrawCreatorSurplusInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	creator solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	quoteMint := config.QuoteMint    // solana.WrappedSol
	baseMint := virtualPool.BaseMint // baseMint
	quoteVault := virtualPool.QuoteVault

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction
	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, creator, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteProgram := dbc.GetTokenProgram(config.QuoteTokenFlag)

	withdrawIx, err := dbcWithdrawCreatorSurplus(m,
		virtualPool.Config,
		pool,
		tokenQuoteAccount,
		quoteVault,
		quoteMint,
		creator,
		tokenQuoteProgram,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, withdrawIx)

	if quoteMint.Equals(solana.WrappedSol) {
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			creator,
			creator,
			[]solana.PublicKey{},
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

func (m *DBC) WithdrawCreatorSurplus(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {

	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", err
	}

	instructions, err := m.WithdrawCreatorSurplusInstruction(ctx, payer.PublicKey(), m.poolCreator.PublicKey(), virtualPool, config)
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

func withdrawMigrationFee(
	m *DBC,
	// Params:
	flag uint8,

	// Accounts:
	config solana.PublicKey,
	dbcPool solana.PublicKey,
	tokenQuoteAccount solana.PublicKey,
	quoteVault solana.PublicKey,
	quoteMint solana.PublicKey,
	sender solana.PublicKey,
	tokenQuoteProgram solana.PublicKey,
) (solana.Instruction, error) {

	poolAuthority := m.poolAuthority
	eventAuthority := m.eventAuthority

	program := dbc.ProgramID

	return dbc.NewWithdrawMigrationFeeInstruction(
		flag,
		// Accounts:
		poolAuthority,
		config,
		dbcPool,
		tokenQuoteAccount,
		quoteVault,
		quoteMint,
		sender,
		tokenQuoteProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) WithdrawMigrationFeeInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	flag uint8,
	account solana.PublicKey,
	virtualPool *dbc.VirtualPool,
	config *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	quoteMint := config.QuoteMint    // solana.WrappedSol
	baseMint := virtualPool.BaseMint // baseMint
	quoteVault := virtualPool.QuoteVault

	pool, err := dbc.DeriveDbcPoolPDA(quoteMint, baseMint, virtualPool.Config)
	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction
	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, m.rpcClient, account, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteProgram := dbc.GetTokenProgram(config.QuoteTokenFlag)

	withdrawIx, err := withdrawMigrationFee(m,
		flag,
		virtualPool.Config,
		pool,
		tokenQuoteAccount,
		quoteVault,
		quoteMint,
		account,
		tokenQuoteProgram,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, withdrawIx)

	if quoteMint.Equals(solana.WrappedSol) {
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			account,
			account,
			[]solana.PublicKey{},
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

func (m *DBC) WithdrawPartnerMigrationFee(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", err
	}

	instructions, err := m.WithdrawMigrationFeeInstruction(ctx, payer.PublicKey(), 0, m.feeClaimer.PublicKey(), virtualPool, config)
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

func (m *DBC) WithdrawCreatorMigrationFee(
	ctx context.Context,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	virtualPool, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	config, err := m.GetConfig(ctx, virtualPool.Config)
	if err != nil {
		return "", err
	}

	instructions, err := m.WithdrawMigrationFeeInstruction(ctx, payer.PublicKey(), 1, m.poolCreator.PublicKey(), virtualPool, config)
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
