package dynamic_bonding_curve

import (
	"bytes"

	"github.com/gagliardetto/solana-go"
	"github.com/shopspring/decimal"
)

// Derives the dbc pool address
func DeriveDbcPoolPDA(quoteMint, baseMint, config solana.PublicKey) (solana.PublicKey, error) {
	// pda order: the larger public key bytes goes first
	var mintA, mintB solana.PublicKey
	if bytes.Compare(quoteMint.Bytes(), baseMint.Bytes()) > 0 {
		mintA = quoteMint
		mintB = baseMint
	} else {
		mintA = baseMint
		mintB = quoteMint
	}
	seeds := [][]byte{
		[]byte("pool"),
		config.Bytes(),
		mintA.Bytes(),
		mintB.Bytes(),
	}
	pda, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		// log.Fatalf("find pool PDA: %v", err)
		return solana.PublicKey{}, err
	}
	return pda, nil
}

// Derives the dbc token vault address
func DeriveTokenVaultPDA(pool, mint solana.PublicKey) (solana.PublicKey, error) {
	seed := [][]byte{
		[]byte("token_vault"),
		mint.Bytes(),
		pool.Bytes(),
	}
	pda, _, err := solana.FindProgramAddress(seed, ProgramID)
	if err != nil {
		// log.Fatalf("find vault PDA: %v", err)
		return solana.PublicKey{}, err
	}
	return pda, nil
}

// Derives the event authority PDA
func DeriveEventAuthorityPDA() (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("__event_authority")}
	address, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		// panic(err)
		return solana.PublicKey{}, err
	}
	return address, nil
}

// Derives the pool authority PDA
func DerivePoolAuthorityPDA() (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("pool_authority")}
	address, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		var x []byte
		return solana.PublicKey(x), err
	}
	return address, nil
}

func DerivePartnerMetadataPDA(feeClaimer solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{
		[]byte("partner_metadata"),
		feeClaimer.Bytes(),
	}
	pda, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		// log.Fatalf("find DAMM V1 migration metadata PDA: %v", err)
		return solana.PublicKey{}, err
	}
	return pda, nil
}

func DeriveDbcPoolMetadataPDA(mint solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{
		[]byte("virtual_pool_metadata"),
		mint.Bytes(),
	}
	pda, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		// log.Fatalf("find DAMM V1 migration metadata PDA: %v", err)
		return solana.PublicKey{}, err
	}
	return pda, nil
}

// Derives the mint metadata address
func DeriveMintMetadataPDA(mint solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{
		[]byte("metadata"),
		solana.TokenMetadataProgramID.Bytes(),
		mint.Bytes(),
	}
	pda, _, err := solana.FindProgramAddress(seeds, solana.TokenMetadataProgramID)
	if err != nil {
		// log.Fatalf("find mint metadata PDA: %v", err)
		return solana.PublicKey{}, err
	}
	return pda, nil
}

// Derives the DAMM V1 migration metadata PDA
func DeriveDammV1MigrationMetadataPDA(pool solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{
		[]byte("meteora"),
		pool.Bytes(),
	}
	pda, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		// log.Fatalf("find DAMM V1 migration metadata PDA: %v", err)
		return solana.PublicKey{}, err
	}
	return pda, nil
}

func DeriveDammV2MigrationMetadataPDA(poolPDA solana.PublicKey) (solana.PublicKey, error) {
	seeds := [][]byte{[]byte("damm_v2"), poolPDA.Bytes()}

	damm_v2, _, err := solana.FindProgramAddress(seeds, ProgramID)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return damm_v2, nil
}

func GetTokenProgram(tokenType TokenType) solana.PublicKey {
	if tokenType == TokenTypeSPL {
		return solana.TokenProgramID
	} else {
		return solana.Token2022ProgramID
	}
}

// checkRateLimiterApplied
func CheckRateLimiterApplied(
	baseFeeMode BaseFeeMode,
	swapBaseForQuote bool,
	currentPoint, activationPoint, maxLimiterDuration decimal.Decimal,
) bool {
	// 1. baseFeeMode == RateLimiter
	// 2. swapBaseForQuote == false
	// 3. currentPoint >= activationPoint
	// 4. currentPoint <= activationPoint + maxLimiterDuration

	if baseFeeMode == BaseFeeModeRateLimiter &&
		!swapBaseForQuote &&
		!currentPoint.LessThan(activationPoint) &&
		!currentPoint.GreaterThan(activationPoint.Add(maxLimiterDuration)) {
		// 	 !decimal.NewFromBigInt(currentPoint, 0).LessThan(decimal.NewFromBigInt(activationPoint, 0)) && // currentPoint >= activationPoint
		// !decimal.NewFromBigInt(currentPoint, 0).GreaterThan(decimal.NewFromBigInt(activationPoint, 0).Add(decimal.NewFromBigInt(maxLimiterDuration, 0))) { // currentPoint <= activationPoint + maxLimiterDuration
		return true
	}

	return false
}
