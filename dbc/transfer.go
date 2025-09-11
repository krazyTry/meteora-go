package dbc

import (
	"context"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc/ws"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func (m *DBC) Transfer(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	sender *solana.Wallet,
	receiver *solana.Wallet,
	baseMint solana.PublicKey,
	amount *big.Int,
) (string, error) {

	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	configState, err := m.GetConfig(ctx, poolState.Config)
	if err != nil {
		return "", err
	}

	instructions, err := solanago.TransferInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		sender.PublicKey(),
		receiver.PublicKey(),
		baseMint,
		uint8(configState.TokenDecimal),
		amount,
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
