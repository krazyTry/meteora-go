package dbc

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	dbc "github.com/krazyTry/meteora-go/dbc/dynamic_bonding_curve"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func CreateConfigInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	config solana.PublicKey,
	partner solana.PublicKey,
	leftoverReceiver solana.PublicKey,
	quoteMint solana.PublicKey,
	configParameters *dbc.ConfigParameters,
) ([]solana.Instruction, error) {

	createConfigIx, err := dbc.NewCreateConfigInstruction(
		configParameters,
		config,
		partner,
		leftoverReceiver,
		quoteMint,
		payer,
		solana.SystemProgramID,
		eventAuthority,
		dbc.ProgramID,
	)

	if err != nil {
		return nil, err
	}
	return []solana.Instruction{createConfigIx}, nil
}

func (m *DBC) CreateConfig(
	ctx context.Context,
	payer *solana.Wallet,
	quoteMint solana.PublicKey,
	cfg *dbc.ConfigParameters,
) (string, error) {

	instructions, err := CreateConfigInstruction(
		ctx,
		payer.PublicKey(),
		m.config.PublicKey(),
		m.feeClaimer.PublicKey(),
		m.leftoverReceiver.PublicKey(),
		quoteMint,
		cfg,
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
			case key.Equals(m.config.PublicKey()):
				return &m.config.PrivateKey
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

func (m *DBC) GetConfig(
	ctx context.Context,
	config solana.PublicKey,
) (*dbc.PoolConfig, error) {
	return GetConfig(ctx, m.rpcClient, config)
}

func GetConfig(
	ctx context.Context,
	rpcClient *rpc.Client,
	config solana.PublicKey,
) (*dbc.PoolConfig, error) {
	out, err := solanago.GetAccountInfo(ctx, rpcClient, config)
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

func (m *DBC) InitConfig(
	ctx context.Context,
	payerWallet *solana.Wallet,
	quoteMint solana.PublicKey,
	cfg *dbc.ConfigParameters,
) error {
	config, err := m.GetConfig(ctx, m.config.PublicKey())
	if err != nil {
		return err
	}

	if config != nil {
		return nil
	}

	if _, err = m.CreateConfig(ctx, payerWallet, quoteMint, cfg); err != nil {
		return err
	}
	return nil
}
