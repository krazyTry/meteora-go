package dbc

import (
	"context"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func WithdrawLeftoverInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	leftoverReceiver solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	baseMint := poolState.BaseMint // baseMint

	var instructions []solana.Instruction

	tokenBaseAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, leftoverReceiver, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	baseVault := poolState.BaseVault // dbc.DeriveTokenVaultPDA(pool, virtualPool.BaseMint)

	tokenBaseProgram := dbc.GetTokenProgram(configState.TokenType)

	withdrawIx, err := dbc.NewWithdrawLeftoverInstruction(
		poolAuthority,
		poolState.Config,
		poolAddress,
		tokenBaseAccount,
		baseVault,
		baseMint,
		leftoverReceiver,
		tokenBaseProgram,
		eventAuthority,
		dbc.ProgramID,
	)
	if err != nil {
		return nil, err
	}

	instructions = append(instructions, withdrawIx)
	return instructions, nil
}

func (m *DBC) WithdrawLeftover(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {

	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", err
	}

	instructions, err := WithdrawLeftoverInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.leftoverReceiver.PublicKey(),
		poolState.Address,
		poolState.VirtualPool,
		configState,
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

func WithdrawPartnerSurplusInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolPartner solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	quoteMint := configState.QuoteMint // solana.WrappedSol
	quoteVault := poolState.QuoteVault

	var instructions []solana.Instruction
	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, poolPartner, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteProgram := dbc.GetTokenProgram(configState.QuoteTokenFlag)

	withdrawIx, err := dbc.NewPartnerWithdrawSurplusInstruction(
		poolAuthority,
		poolState.Config,
		poolAddress,
		tokenQuoteAccount,
		quoteVault,
		quoteMint,
		poolPartner,
		tokenQuoteProgram,
		eventAuthority,
		dbc.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, withdrawIx)

	if quoteMint.Equals(solana.WrappedSol) {
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

func (m *DBC) WithdrawPartnerSurplus(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", err
	}

	instructions, err := WithdrawPartnerSurplusInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.feeClaimer.PublicKey(),
		poolState.Address,
		poolState.VirtualPool,
		configState,
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

func WithdrawCreatorSurplusInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	config *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	quoteMint := config.QuoteMint // solana.WrappedSol
	quoteVault := poolState.QuoteVault

	var instructions []solana.Instruction
	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, poolCreator, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteProgram := dbc.GetTokenProgram(config.QuoteTokenFlag)

	withdrawIx, err := dbc.NewCreatorWithdrawSurplusInstruction(
		poolAuthority,
		poolState.Config,
		poolAddress,
		tokenQuoteAccount,
		quoteVault,
		quoteMint,
		poolCreator,
		tokenQuoteProgram,
		eventAuthority,
		dbc.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, withdrawIx)

	if quoteMint.Equals(solana.WrappedSol) {
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

func (m *DBC) WithdrawCreatorSurplus(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {

	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", err
	}

	instructions, err := WithdrawCreatorSurplusInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		poolState.Address,
		poolState.VirtualPool,
		configState,
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

func WithdrawMigrationFeeInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	owner solana.PublicKey,
	flag uint8,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	quoteMint := configState.QuoteMint // solana.WrappedSol
	quoteVault := poolState.QuoteVault

	var instructions []solana.Instruction
	tokenQuoteAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, owner, quoteMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	tokenQuoteProgram := dbc.GetTokenProgram(configState.QuoteTokenFlag)

	withdrawIx, err := dbc.NewWithdrawMigrationFeeInstruction(
		flag,
		// Accounts:
		poolAuthority,
		poolState.Config,
		poolAddress,
		tokenQuoteAccount,
		quoteVault,
		quoteMint,
		owner,
		tokenQuoteProgram,
		eventAuthority,
		dbc.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	instructions = append(instructions, withdrawIx)

	if quoteMint.Equals(solana.WrappedSol) {
		closeWSOLIx := token.NewCloseAccountInstruction(
			tokenQuoteAccount,
			owner,
			owner,
			[]solana.PublicKey{},
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

func (m *DBC) WithdrawPartnerMigrationFee(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", err
	}

	instructions, err := WithdrawMigrationFeeInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.feeClaimer.PublicKey(),
		0,

		poolState.Address,
		poolState.VirtualPool,
		configState,
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

func (m *DBC) WithdrawCreatorMigrationFee(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", err
	}

	instructions, err := WithdrawMigrationFeeInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		m.poolCreator.PublicKey(),
		1,
		poolState.Address,
		poolState.VirtualPool,
		configState,
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
