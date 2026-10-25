package helpers

import (
	"math/big"

	"github.com/krazyTry/meteora-go/dynamic_bonding_curve/shared"
)

// SurplusBreakdown is the quote reserve above the migration threshold.
// Partner and creator share PartnerAndCreatorSurplusShare percent, split by
// the creator trading fee percentage with the creator rounded down.
type SurplusBreakdown struct {
	Total    *big.Int
	Partner  *big.Int
	Creator  *big.Int
	Protocol *big.Int
}

// GetSurplusBreakdown matches the program's partner and creator surplus payout.
// Every amount is zero before the curve completes.
func GetSurplusBreakdown(quoteReserve, migrationQuoteThreshold uint64, creatorTradingFeePercentage uint8) SurplusBreakdown {
	zero := big.NewInt(0)
	if quoteReserve < migrationQuoteThreshold {
		return SurplusBreakdown{Total: zero, Partner: zero, Creator: zero, Protocol: zero}
	}
	total := new(big.Int).SetUint64(quoteReserve - migrationQuoteThreshold)
	partnerAndCreator := new(big.Int).Mul(total, big.NewInt(shared.PartnerAndCreatorSurplusShare))
	partnerAndCreator.Div(partnerAndCreator, big.NewInt(100))
	creator := new(big.Int).Mul(partnerAndCreator, big.NewInt(int64(creatorTradingFeePercentage)))
	creator.Div(creator, big.NewInt(100))
	return SurplusBreakdown{
		Total:    total,
		Partner:  new(big.Int).Sub(partnerAndCreator, creator),
		Creator:  creator,
		Protocol: new(big.Int).Sub(total, partnerAndCreator),
	}
}
