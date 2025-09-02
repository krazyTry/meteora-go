package dbc

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func dbcCreateConfig(m *DBC,
	config,
	feeClaimer,
	leftoverReceiver,
	payer,
	quoteMint solana.PublicKey,
	configParameters *dbc.ConfigParameters,
) (solana.Instruction, error) {

	eventAuthority := m.eventAuthority

	systemProgram := solana.SystemProgramID

	// quoteMint := solana.WrappedSol

	program := dbc.ProgramID

	if configParameters == nil {
		return nil, fmt.Errorf("configParameters is nil")
	}

	return dbc.NewCreateConfigInstruction(
		configParameters,
		config,
		feeClaimer,
		leftoverReceiver,
		quoteMint,
		payer,
		systemProgram,
		eventAuthority,
		program,
	)
}

func (m *DBC) CreateConfig(
	ctx context.Context,
	payer *solana.Wallet,
	config *solana.Wallet,
	partner solana.PublicKey,
	leftoverReceiver solana.PublicKey,
	quoteMint solana.PublicKey,
	cfg *dbc.ConfigParameters,
) (string, error) {

	createConfigIx, err := dbcCreateConfig(
		m,
		m.config.PublicKey(),
		partner,
		leftoverReceiver,
		payer.PublicKey(),
		quoteMint,
		cfg,
	)
	if err != nil {
		return "", err
	}

	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction([]solana.Instruction{createConfigIx}, latestBlockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(payer.PublicKey()):
			return &payer.PrivateKey
		case key.Equals(m.config.PublicKey()):
			return &m.config.PrivateKey
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

func (m *DBC) GetConfig(ctx context.Context, config solana.PublicKey) (*dbc.PoolConfig, error) {
	out, err := solanago.GetAccountInfo(ctx, m.rpcClient, config)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	obj, err := dbc.ParseAnyAccount(out.GetBinary())
	if err != nil {
		return nil, err
	}

	cfg, ok := obj.(*dbc.PoolConfig)
	if !ok {
		return nil, fmt.Errorf("obj.(*dbc.PoolConfig) fail")
	}

	return cfg, nil
}

func (m *DBC) InitConfig(ctx context.Context, payerWallet *solana.Wallet, quoteMint solana.PublicKey, cfg *dbc.ConfigParameters) error {
	config, err := m.GetConfig(ctx, m.config.PublicKey())
	if err != nil {
		return err
	}

	if config != nil {
		return nil
	}

	if _, err = m.CreateConfig(ctx, payerWallet, m.config, m.feeClaimer.PublicKey(), m.leftoverReceiver.PublicKey(), quoteMint, cfg); err != nil {
		return err
	}
	return nil
}
