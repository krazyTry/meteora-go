package helpers

type TokenType = uint8

const (
	TokenTypeSPL       TokenType = 0
	TokenTypeToken2022 TokenType = 1
)

type TokenDecimal = uint8

const (
	TokenDecimalSix   TokenDecimal = 6
	TokenDecimalSeven TokenDecimal = 7
	TokenDecimalEight TokenDecimal = 8
	TokenDecimalNine  TokenDecimal = 9
)
