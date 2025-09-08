package solana

import (
	"context"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

func TransferInstruction(
	ctx context.Context,
	rpcClient *rpc.Client,
	payer solana.PublicKey,
	sender solana.PublicKey,
	receiver solana.PublicKey,
	mint solana.PublicKey,
	decimals uint8,
	amount *big.Int,
) ([]solana.Instruction, error) {

	var instructions []solana.Instruction

	sendTokenAccount, err := PrepareTokenATA(ctx, rpcClient, sender, mint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	receiveTokenAccount, err := PrepareTokenATA(ctx, rpcClient, receiver, mint, payer, &instructions)
	if err != nil {
		return nil, err
	}

	transferIx := token.NewTransferCheckedInstruction(
		amount.Uint64(),
		decimals,
		sendTokenAccount,
		mint,
		receiveTokenAccount,
		payer,
		[]solana.PublicKey{},
	).Build()

	return append(instructions, transferIx), nil
}
