package dynamic_bonding_curve

import (
	"github.com/krazyTry/meteora-go/locker/locker"

	"github.com/gagliardetto/solana-go"
)

func DeriveLockerEventAuthority() (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("__event_authority")}

	eventAuthority, _, err := solana.FindProgramAddress(seeds, locker.ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return eventAuthority, nil
}

func DeriveBaseKeyForLocker(poolPDA solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("base_locker"), poolPDA.Bytes()}

	baseKey, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return baseKey, nil
}

func DeriveEscrow(base solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{
		[]byte("escrow"),
		base.Bytes(),
	}

	escrow, _, err := solana.FindProgramAddress(seeds, locker.ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return escrow, nil
}
