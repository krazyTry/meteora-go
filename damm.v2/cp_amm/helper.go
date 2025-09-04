package cp_amm

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/gagliardetto/solana-go"
)

// DeriveConfigAddress https://docs.meteora.ag/developer-guide/guides/damm-v2/pool-fee-configs#view-all-public-config-key-addresses-json
func DeriveConfigAddress(index uint64) (solana.PublicKey, error) {
	indexBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(indexBytes, index)

	seeds := [][]byte{[]byte("config"), indexBytes}

	pda, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return pda, nil
}

// derivePoolAddress
func DeriveCpAmmPoolPDA(config, baseMint, quoteMint solana.PublicKey) (solana.PublicKey, error) {
	var mintA, mintB solana.PublicKey
	if bytes.Compare(quoteMint.Bytes(), baseMint.Bytes()) > 0 {
		mintA = quoteMint
		mintB = baseMint
	} else {
		mintA = baseMint
		mintB = quoteMint
	}
	seeds := [][]byte{[]byte("pool"), config.Bytes(), mintA.Bytes(), mintB.Bytes()}

	address, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		// log.Fatal(err)
		return solana.PublicKey{}, err
	}
	return address, nil
}

func DeriveCustomizablePoolAddress(baseMint, quoteMint solana.PublicKey) (solana.PublicKey, error) {
	var mintA, mintB solana.PublicKey
	if bytes.Compare(quoteMint.Bytes(), baseMint.Bytes()) > 0 {
		mintA = quoteMint
		mintB = baseMint
	} else {
		mintA = baseMint
		mintB = quoteMint
	}
	seeds := [][]byte{[]byte("cpool"), mintA.Bytes(), mintB.Bytes()}
	pda, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return pda, nil
}

func DerivePositionNftAccount(positionNftMint solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{
		[]byte("position_nft_account"),
		positionNftMint.Bytes(),
	}
	pda, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return pda, nil
}

func DerivePoolAuthorityPDA() (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("pool_authority")}
	address, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return address, nil
}

// Derives the event authority PDA
func DeriveEventAuthorityPDA() (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("__event_authority")}
	address, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return address, nil
}

func DerivePositionAddress(positionNft solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("position"), positionNft.Bytes()}

	pda, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return pda, nil
}

func DeriveTokenVaultAddress(tokenMint, pool solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("token_vault"), tokenMint.Bytes(), pool.Bytes()}

	pda, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return pda, nil
}

// DeriveTokenBadgeAddress derives the PDA for a token badge.
func DeriveTokenBadgeAddress(tokenMint solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("token_badge"), tokenMint.Bytes()}

	pda, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return pda, nil
}

func GetTokenProgram(tokenType TokenType) solana.PublicKey {
	if tokenType == TokenTypeSPL {
		return solana.TokenProgramID
	} else {
		return solana.Token2022ProgramID
	}
}

func IsVestingComplete(cliffPoint, periodFrequency *big.Int, numberOfPeriods uint16, currentPoint *big.Int) bool {
	// endPoint = cliffPoint + periodFrequency * numberOfPeriods
	endPoint := new(big.Int).Mul(periodFrequency, big.NewInt(int64(numberOfPeriods)))
	endPoint.Add(endPoint, cliffPoint)

	// return currentPoint >= endPoint
	return currentPoint.Cmp(endPoint) >= 0
}

// initialPoolTokenAmount = tokenAmount * 10^decimals
func GetInitialPoolTokenAmount(amount *big.Int, decimals uint8) *big.Int {

	// 10^decimals
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	// tokenAmount * 10^decimals
	result := new(big.Int).Mul(amount, scale)

	return result
}
