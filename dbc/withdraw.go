package dbc

import (
	"context"
	"fmt"

	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	solanago "github.com/krazyTry/meteora-go/solana"
)

// WithdrawLeftoverInstruction generates the instruction needed to withdraw leftover tokens.
//
// Example:
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// configState, _ := m.GetConfig(ctx, poolState.Config)
//
// instructions, _ := WithdrawLeftoverInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	m.leftoverReceiver.PublicKey(), // leftover receiver account
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool,// dbc pool state
//	configState, // dbc pool config
//
// )
func WithdrawLeftoverInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	leftoverReceiver solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	if poolState.IsWithdrawLeftover == 1 {
		return nil, fmt.Errorf("withdrawLeftover has been claimed")
	}

	baseMint := poolState.BaseMint

	var instructions []solana.Instruction

	tokenBaseAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, leftoverReceiver, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	baseVault := poolState.BaseVault

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

// WithdrawLeftover withdraws leftover tokens from the Dynamic Bonding Curve pool.
// It depends on the WithdrawLeftoverInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// instructions, _ := WithdrawLeftover(
//
//	ctx,
//	wsClient,
//	payer, // payer account
//	baseMint, // pool (token) address
//
// )
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

// WithdrawPartnerSurplusInstruction generates the instruction needed to withdraw partner surplus.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// configState, _ := m.GetConfig(ctx, poolState.Config)
//
// instructions, _ := WithdrawPartnerSurplusInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	m.feeClaimer.PublicKey(), // partner
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//
// )
func WithdrawPartnerSurplusInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolPartner solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	configState *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	if poolState.IsPartnerWithdrawSurplus == 1 {
		return nil, fmt.Errorf("partnerWithdrawSurplus has been claimed")
	}

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

// WithdrawPartnerSurplus withdraws the partner’s surplus from the pool.
// It depends on the WithdrawPartnerSurplusInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// sig, _ := meteoraDBC.WithdrawPartnerSurplus(
//
//	ctx,
//	wsClient,
//	payer, // payer account
//	baseMint,
//
// )
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

// WithdrawCreatorSurplusInstruction generates the instruction needed to withdraw creator surplus.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// configState, _ := m.GetConfig(ctx, poolState.Config)
//
// instructions, _ := WithdrawCreatorSurplusInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	m.poolCreator.PublicKey(), // creator
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//
// )
func WithdrawCreatorSurplusInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	poolCreator solana.PublicKey,
	poolAddress solana.PublicKey,
	poolState *dbc.VirtualPool,
	config *dbc.PoolConfig,
) ([]solana.Instruction, error) {
	if poolState.IsCreatorWithdrawSurplus == 1 {
		return nil, fmt.Errorf("creatorWithdrawSurplus has been claimed")
	}

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
			nil,
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

// WithdrawCreatorSurplus withdraws the creator’s surplus from the pool.
// It depends on the WithdrawCreatorSurplusInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// sig, _ := meteoraDBC.WithdrawCreatorSurplus(
//
//	ctx,
//	wsClient,
//	payer, // payer account
//	baseMint,
//
// )
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

// WithdrawMigrationFeeInstruction generates the instruction needed to withdraw the migration fee.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// poolState, _ := m.GetPoolByBaseMint(ctx, baseMint)
//
// configState, _ := m.GetConfig(ctx, poolState.Config)
//
// instructions, _ := WithdrawMigrationFeeInstruction(
//
//	ctx,
//	m.rpcClient,
//	payer.PublicKey(), // payer account
//	m.feeClaimer.PublicKey(), // partner
//	0, // 0. partner 1. creator
//	poolState.Address, // dbc pool address
//	poolState.VirtualPool, // dbc pool state
//	configState, // dbc pool config
//
// )
func WithdrawMigrationFeeInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	owner solana.PublicKey,
	flag dbc.WithdrawMigrationFeeFlag, // 0. partner 1. creator
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
			nil,
		).Build()

		instructions = append(instructions, closeWSOLIx)
	}
	return instructions, nil
}

// WithdrawPartnerMigrationFee withdraws the partner’s migration fee from the pool.
// It depends on the WithdrawMigrationFeeInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// sig, _ := meteoraDBC.WithdrawPartnerMigrationFee(
//
//	ctx,
//	wsClient,
//	payer, // payer account
//	baseMint,
//
// )
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
		dbc.PartnerWithdrawMigrationFeeFlag,

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

// WithdrawCreatorMigrationFee withdraws the creator’s migration fee from the pool.
// It depends on the WithdrawMigrationFeeInstruction function.
// This function is blocking and will wait for on-chain confirmation before returning.
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// sig, _ := meteoraDBC.WithdrawCreatorMigrationFee(
//
//	ctx,
//	wsClient,
//	payer, // payer account
//	baseMint,
//
// )
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
		dbc.CreatorWithdrawMigrationFeeFlag,
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
