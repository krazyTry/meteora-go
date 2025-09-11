package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc/ws"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func (m *DammV2) Transfer(
	ctx context.Context,
	wsClient *ws.Client,
	payer *solana.Wallet,
	sender *solana.Wallet,
	receiver *solana.Wallet,
	baseMint solana.PublicKey,
	amount *big.Int,
) (string, error) {

	tokens, err := solanago.GetMultipleToken(ctx, m.rpcClient, baseMint)
	if err != nil {
		return "", err
	}
	if tokens[0] == nil {
		return "", fmt.Errorf("baseMint error")
	}

	instructions, err := solanago.TransferInstruction(
		ctx,
		m.rpcClient,
		payer.PublicKey(),
		sender.PublicKey(),
		receiver.PublicKey(),
		baseMint,
		tokens[0].Decimals,
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
