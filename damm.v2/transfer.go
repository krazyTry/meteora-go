package dammV2

import (
	"context"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc/ws"
	solanago "github.com/krazyTry/meteora-go/solana"
)

// Transfer transfers tokens from a Damm v2 pool.
// This function is blocking and will wait for on-chain confirmation before returning.
// This function is an example function
//
// Example:
//
// baseMint := solana.MustPublicKeyFromBase58("BHyqU2m7YeMFM3PaPXd2zdk7ApVtmWVsMiVK148vxRcS")
//
// sig, _ := meteoraDBC.Transfer(
//
//	ctx,
//	wsClient,
//	payer, // payer account
//	from, // sender account
//	to, // receiver account
//	baseMint,// token address
//	amount,// transfer amount
//
// )
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
