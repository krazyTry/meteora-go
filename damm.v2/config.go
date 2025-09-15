package dammV2

import (
	"context"
	"fmt"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// func cpAmmCreateConfig(
// 	m *DammV2,
// 	// Params:
// 	idx uint64,
// 	configParameters *cp_amm.StaticConfigParameters,

// 	// Accounts:
// 	config solana.PublicKey,
// 	admin solana.PublicKey,
// ) (solana.Instruction, error) {

// 	return cp_amm.NewCreateConfigInstruction(
// 		idx,
// 		configParameters,
// 		config,
// 		admin,
// 		solana.SystemProgramID,
// 		eventAuthority,
// 		cp_amm.ProgramID,
// 	)
// }

// // CreateStaticConfig
// func (m *DammV2) CreateStaticConfig(
// 	ctx context.Context,
// 	payer *solana.Wallet,
// 	configIDX uint64,
// 	config *cp_amm.StaticConfigParameters,
// ) (string, solana.PublicKey, error) {
// 	configPDA, err := cp_amm.DeriveConfigAddress(configIDX)
// 	if err != nil {
// 		return "", solana.PublicKey{}, err
// 	}
// 	admin := solana.MustPublicKeyFromBase58("5unTfT2kssBuNvHPY6LbJfJpLqEcdMxGYLWHwShaeTLi")
// 	configIx, err := cpAmmCreateConfig(m, configIDX, config, configPDA, admin)
// 	if err != nil {
// 		return "", solana.PublicKey{}, err
// 	}
// 	sig, err := solanago.SendTransaction(ctx,
// 		m.rpcClient,
// 		m.wsClient,
// 		[]solana.Instruction{configIx},
// 		payer.PublicKey(),
// 		func(key solana.PublicKey) *solana.PrivateKey {
// 			switch {
// 			case key.Equals(payer.PublicKey()):
// 				return &payer.PrivateKey
// 			default:
// 				return nil
// 			}
// 		},
// 	)
// 	if err != nil {
// 		return "", solana.PublicKey{}, err
// 	}
// 	return sig.String(), configPDA, nil
// }

// func cpAmmCreateDynamicConfig(
// 	m *DammV2,
// 	// Params:
// 	idx uint64,
// 	configParameters *cp_amm.DynamicConfigParameters,

// 	// Accounts:
// 	config solana.PublicKey,
// 	admin solana.PublicKey,
// ) (solana.Instruction, error) {

// 	return cp_amm.NewCreateDynamicConfigInstruction(
// 		idx,
// 		configParameters,
// 		config,
// 		admin,
// 		solana.SystemProgramID,
// 		eventAuthority,
// 		cp_amm.ProgramID,
// 	)
// }

// // CreateDynamicConfig
// func (m *DammV2) CreateDynamicConfig(
// 	ctx context.Context,
// 	payer *solana.Wallet,
// 	configIDX uint64,
// 	config *cp_amm.DynamicConfigParameters,
// ) (string, solana.PublicKey, error) {
// 	configPDA, err := cp_amm.DeriveConfigAddress(configIDX)
// 	if err != nil {
// 		return "", solana.PublicKey{}, err
// 	}
// 	admin := solana.MustPublicKeyFromBase58("5unTfT2kssBuNvHPY6LbJfJpLqEcdMxGYLWHwShaeTLi")
// 	configIx, err := cpAmmCreateDynamicConfig(m, configIDX, config, configPDA, admin)
// 	if err != nil {
// 		return "", solana.PublicKey{}, err
// 	}
// 	sig, err := solanago.SendTransaction(ctx,
// 		m.rpcClient,
// 		m.wsClient,
// 		[]solana.Instruction{configIx},
// 		payer.PublicKey(),
// 		func(key solana.PublicKey) *solana.PrivateKey {
// 			switch {
// 			case key.Equals(payer.PublicKey()):
// 				return &payer.PrivateKey
// 			default:
// 				return nil
// 			}
// 		},
// 	)
// 	if err != nil {
// 		return "", solana.PublicKey{}, err
// 	}
// 	return sig.String(), configPDA, nil
// }

// func cpAmmCloseConfig(
// 	m *DammV2,
// 	config solana.PublicKey,
// 	admin solana.PublicKey,
// 	rentReceiver solana.PublicKey,
// ) (solana.Instruction, error) {

// 	return cp_amm.NewCloseConfigInstruction(
// 		config,
// 		admin,
// 		rentReceiver,
// 		eventAuthority,
// 		cp_amm.ProgramID,
// 	)
// }

// // CloseConfig admin is the paying account and also an important parameter that needs to be provided when closing the config.
// func (m *DammV2) CloseConfig(
// 	ctx context.Context,
// 	payer *solana.Wallet,
// 	configIDX uint64,
// ) (string, error) {
// 	configPDA, err := cp_amm.DeriveConfigAddress(configIDX)
// 	if err != nil {
// 		return "", err
// 	}
// 	admin := solana.MustPublicKeyFromBase58("5unTfT2kssBuNvHPY6LbJfJpLqEcdMxGYLWHwShaeTLi")
// 	configIx, err := cpAmmCloseConfig(m, configPDA, admin, payer.PublicKey())
// 	if err != nil {
// 		return "", err
// 	}
// 	sig, err := solanago.SendTransaction(ctx,
// 		m.rpcClient,
// 		m.wsClient,
// 		[]solana.Instruction{configIx},
// 		payer.PublicKey(),
// 		func(key solana.PublicKey) *solana.PrivateKey {
// 			switch {
// 			case key.Equals(payer.PublicKey()):
// 				return &payer.PrivateKey
// 			default:
// 				return nil
// 			}
// 		},
// 	)
// 	if err != nil {
// 		return "", err
// 	}
// 	return sig.String(), nil
// }

// GetConfig Fetches the Config state of the program.
//
// Example:
//
// config, _ := meteoraDammV2.GetConfig(ctx, solana.MustPublicKeyFromBase58("82p7sVzQWZfCrmStPhsG8BYKwheQkUiXSs2wiqdhwNxr"))
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
