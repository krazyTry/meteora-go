package solana

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
)

type Token struct {
	token.Mint
	Owner solana.PublicKey
}

type TokenLayout struct {
}

func (l *TokenLayout) Decode(data []byte) (*Token, error) {
	mint := token.Mint{}

	if err := mint.Decode(data); err != nil {
		return nil, err
	}
	return &Token{Mint: mint}, nil
}
