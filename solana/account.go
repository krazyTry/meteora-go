package solana

import (
	binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

// tokenAccountLayout represents the raw token account layout structure
// Reference implementation: https://github.com/solana-labs/solana-program-library/blob/d72289c79a04411c69a8bf1054f7156b6196f9b3/token/js/src/state/account.ts#L69
type tokenAccountLayout struct {
	Mint                 solana.PublicKey  // Token mint address
	Owner                solana.PublicKey  // Account owner address
	Amount               uint64            // Token amount held in the account
	DelegateOption       uint32            // Option flag for delegate presence
	Delegate             *solana.PublicKey // Delegate authority (optional)
	State                uint8             // Account state (initialized, frozen, etc.)
	IsNativeOption       uint32            // Option flag for native token
	IsNative             *uint64           // Native token amount (optional)
	DelegatedAmount      uint64            // Amount delegated to delegate authority
	CloseAuthorityOption uint32            // Option flag for close authority presence
	CloseAuthority       *solana.PublicKey // Close authority (optional)
}

// AccountLayout provides methods for decoding token account data
type AccountLayout struct {
}

func (l *AccountLayout) Decode(data []byte) (*Account, error) {
	rawAccount := &tokenAccountLayout{}
	if err := binary.NewBinDecoder(data).Decode(rawAccount); err != nil {
		return nil, err
	}
	return &Account{
		Mint:   rawAccount.Mint,
		Owner:  rawAccount.Owner,
		Amount: rawAccount.Amount,
		Delegate: func() *solana.PublicKey {
			if rawAccount.DelegateOption > 0 {
				return rawAccount.Delegate
			}
			return nil
		}(),
		DelegatedAmount: rawAccount.DelegatedAmount,
		IsInitialized:   AccountState(rawAccount.State) != AccountStateUninitialized,
		IsFrozen:        AccountState(rawAccount.State) == AccountStateFrozen,
		IsNative:        rawAccount.IsNativeOption > 0,
		RentExemptReserve: func() *uint64 {
			if rawAccount.IsNativeOption > 0 {
				return rawAccount.IsNative
			}
			return nil
		}(),
		CloseAuthority: func() *solana.PublicKey {
			if rawAccount.CloseAuthorityOption > 0 {
				return rawAccount.CloseAuthority
			}
			return nil
		}(),
	}, nil
}

// AccountState represents the state of a token account
type AccountState uint8

const (
	AccountStateUninitialized AccountState = 0
	AccountStateInitialized   AccountState = 1
	AccountStateFrozen        AccountState = 2
)

// Account represents a Solana token account with all its properties
type Account struct {
	Address solana.PublicKey // Account address
	// Mint associated with the account
	Mint solana.PublicKey

	// Owner of the account
	Owner solana.PublicKey

	// Number of tokens the account holds
	Amount uint64

	// Authority that can transfer tokens from the account
	Delegate *solana.PublicKey

	// Number of tokens the delegate is authorized to transfer
	DelegatedAmount uint64

	// True if the account is initialized
	IsInitialized bool

	// True if the account is frozen
	IsFrozen bool

	// True if the account is a native token account
	IsNative bool

	// If the account is a native token account, it must be rent-exempt.
	// The rent-exempt reserve is the amount that must remain in the balance until the account is closed.
	RentExemptReserve *uint64

	// Optional authority to close the account
	CloseAuthority *solana.PublicKey
}
