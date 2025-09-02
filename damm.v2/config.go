package dammV2

import (
	"context"
	"fmt"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	sendandconfirmtransaction "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
)

type CpAmmStaticConfigParameters = cp_amm.StaticConfigParameters
type CpAmmDynamicConfigParameters = cp_amm.DynamicConfigParameters

func cpAmmCreateConfig(m *DammV2,
	// Params:
	idx uint64,
	configParameters *cp_amm.StaticConfigParameters,

	// Accounts:
	config solana.PublicKey,
	admin solana.PublicKey,
) (solana.Instruction, error) {

	eventAuthority := m.eventAuthority
	systemProgram := solana.SystemProgramID
	program := cp_amm.ProgramID

	return cp_amm.NewCreateConfigInstruction(
		idx,
		configParameters,
		config,
		admin,
		systemProgram,
		eventAuthority,
		program,
	)
}

// CreateStaticConfig admin is the paying account and also an important parameter that needs to be provided when closing the config.
func (m *DammV2) CreateStaticConfig(ctx context.Context, publicConfigIDX uint64, config *CpAmmStaticConfigParameters, adminWallet *solana.Wallet) (string, error) {
	configIx, err := cpAmmCreateConfig(m, publicConfigIDX, config, m.config.PublicKey(), adminWallet.PublicKey())
	if err != nil {
		return "", err
	}
	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction([]solana.Instruction{configIx}, latestBlockhash, solana.TransactionPayer(adminWallet.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(adminWallet.PublicKey()):
			return &adminWallet.PrivateKey
		case key.Equals(m.config.PublicKey()):
			return &m.config.PrivateKey
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

func cpAmmCreateDynamicConfig(m *DammV2,
	// Params:
	idx uint64,
	configParameters *cp_amm.DynamicConfigParameters,

	// Accounts:
	config solana.PublicKey,
	admin solana.PublicKey,
) (solana.Instruction, error) {

	eventAuthority := m.eventAuthority
	systemProgram := solana.SystemProgramID
	program := cp_amm.ProgramID

	return cp_amm.NewCreateDynamicConfigInstruction(
		idx,
		configParameters,
		config,
		admin,
		systemProgram,
		eventAuthority,
		program,
	)
}

// CreateDynamicConfig admin is the paying account and also an important parameter that needs to be provided when closing the config.
func (m *DammV2) CreateDynamicConfig(ctx context.Context, publicConfigIDX uint64, config *CpAmmDynamicConfigParameters, adminWallet *solana.Wallet) (string, error) {
	configIx, err := cpAmmCreateDynamicConfig(m, publicConfigIDX, config, m.config.PublicKey(), adminWallet.PublicKey())
	if err != nil {
		return "", err
	}

	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction([]solana.Instruction{configIx}, latestBlockhash, solana.TransactionPayer(adminWallet.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(adminWallet.PublicKey()):
			return &adminWallet.PrivateKey
		case key.Equals(m.config.PublicKey()):
			return &m.config.PrivateKey
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

func cpAmmCloseConfig(m *DammV2,
	config solana.PublicKey,
	admin solana.PublicKey,
	rentReceiver solana.PublicKey,
) (solana.Instruction, error) {

	eventAuthority := m.eventAuthority
	program := cp_amm.ProgramID
	return cp_amm.NewCloseConfigInstruction(
		config,
		admin,
		rentReceiver,
		eventAuthority,
		program,
	)
}

// CloseConfig admin is the paying account and also an important parameter that needs to be provided when closing the config.
func (m *DammV2) CloseConfig(ctx context.Context, adminWallet *solana.Wallet) (string, error) {
	configIx, err := cpAmmCloseConfig(m, m.config.PublicKey(), adminWallet.PublicKey(), adminWallet.PublicKey())
	if err != nil {
		return "", err
	}
	latestBlockhash, err := solanago.GetLatestBlockhash(ctx, m.rpcClient)
	if err != nil {
		return "", err
	}

	tx, err := solana.NewTransaction([]solana.Instruction{configIx}, latestBlockhash, solana.TransactionPayer(adminWallet.PublicKey()))
	if err != nil {
		return "", err
	}

	if _, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		switch {
		case key.Equals(adminWallet.PublicKey()):
			return &adminWallet.PrivateKey
		case key.Equals(m.config.PublicKey()):
			return &m.config.PrivateKey
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

func (m *DammV2) GetConfig(ctx context.Context, config solana.PublicKey) (*cp_amm.Config, error) {
	out, err := solanago.GetAccountInfo(ctx, m.rpcClient, config)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	obj, err := cp_amm.ParseAnyAccount(out.GetBinary())
	if err != nil {
		return nil, err
	}

	cfg, ok := obj.(*cp_amm.Config)
	if !ok {
		return nil, fmt.Errorf("obj.(*cp_amm.Config) fail")
	}

	return cfg, nil
}
