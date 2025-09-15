package solana

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
)

// Token represents a Solana token with mint information and owner
type Token struct {
	token.Mint
	// Owner account of the token
	Owner solana.PublicKey
}

// TokenLayout provides methods for decoding token data
type TokenLayout struct {
}

func (l *TokenLayout) Decode(data []byte) (*Token, error) {
	mint := token.Mint{}

	if err := mint.Decode(data); err != nil {
		return nil, err
	}
	return &Token{Mint: mint}, nil
}
