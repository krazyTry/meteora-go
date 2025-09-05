package dbc

import (
	"context"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	solanago "github.com/krazyTry/meteora-go/solana"
)

func TransferInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	sender solana.PublicKey,
	receiver solana.PublicKey,
	baseMint solana.PublicKey,
	baseTokenDecimal uint8,
	amount *big.Int,
) ([]solana.Instruction, error) {

	var instructions []solana.Instruction

	sendTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, sender, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	receiveTokenAccount, err := solanago.PrepareTokenATA(ctx, rpcClient, receiver, baseMint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	transferIx := token.NewTransferCheckedInstruction(
		amount.Uint64(),
		baseTokenDecimal,
		sendTokenAccount,
		baseMint,
		receiveTokenAccount,
		payer,
		[]solana.PublicKey{},
	).Build()

	return append(instructions, transferIx), nil
}

func (m *DBC) Transfer(
	ctx context.Context,
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

	instructions, err := TransferInstruction(
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
		m.wsClient,
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
