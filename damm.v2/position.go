package dammV2

import (
	"context"
	"fmt"
	"maps"
	"math/big"
	"slices"

	"github.com/krazyTry/meteora-go/damm.v2/cp_amm"
	solanago "github.com/krazyTry/meteora-go/solana"
	"github.com/krazyTry/meteora-go/u128"
	"github.com/shopspring/decimal"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func CreatePositionInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	owner solana.PublicKey,
	ownerPositionNft solana.PublicKey,
	poolAddress solana.PublicKey,
) ([]solana.Instruction, error) {

	position, err := cp_amm.DerivePositionAddress(ownerPositionNft)
	if err != nil {
		return nil, err
	}

	positionNftAccount, err := cp_amm.DerivePositionNftAccount(ownerPositionNft)
	if err != nil {
		return nil, err
	}

	createIx, err := cp_amm.NewCreatePositionInstruction(
		owner,
		ownerPositionNft,
		positionNftAccount,
		poolAddress,
		position,
		poolAuthority,
		payer,
		solana.Token2022ProgramID,
		solana.SystemProgramID,
		eventAuthority,
		cp_amm.ProgramID,
	)

	if err != nil {
		return nil, err
	}
	return []solana.Instruction{createIx}, nil
}

func (m *DammV2) CreatePosition(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
) (string, *solana.Wallet, error) {

	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", nil, err
	}

	positionNft := solana.NewWallet()

	instructions, err := CreatePositionInstruction(
		ctx,
		payer.PublicKey(),
		owner.PublicKey(),
		positionNft.PublicKey(),
		poolState.Address,
	)

	if err != nil {
		return "", nil, err
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
			case key.Equals(positionNft.PublicKey()):
				return &positionNft.PrivateKey
			default:
				return nil
			}
		},
	)
	if err != nil {
		return "", nil, err
	}
	return sig.String(), positionNft, nil
}

func ClosePositionInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	owner solana.PublicKey,
	ownerPosition *UserPosition,
	poolAddress solana.PublicKey,
) ([]solana.Instruction, error) {

	closeIx, err := cp_amm.NewClosePositionInstruction(
		ownerPosition.PositionState.NftMint,
		ownerPosition.PositionNftAccount,
		poolAddress,
		ownerPosition.Position,
		poolAuthority,
		payer,
		owner,
		solana.Token2022ProgramID,
		eventAuthority,
		cp_amm.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	return []solana.Instruction{closeIx}, nil
}

func (m *DammV2) ClosePosition(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
) (string, error) {
	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, poolState.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := ClosePositionInstruction(
		ctx,
		payer.PublicKey(),
		owner.PublicKey(),
		userPosition,
		poolState.Address,
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
			case key.Equals(owner.PublicKey()):
				return &owner.PrivateKey
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

func LockPositionInstruction(
	ctx context.Context,
	payer solana.PublicKey,
	owner solana.PublicKey,
	ownerPosition *UserPosition,
	poolAddress solana.PublicKey,
	cliffPoint *uint64,
	periodFrequency uint64,
	cliffUnlockLiquidity *big.Int,
	liquidityPerPeriod *big.Int,
	numberOfPeriod uint16,
	vesting solana.PublicKey,
) ([]solana.Instruction, error) {

	lockIx, err := cp_amm.NewLockPositionInstruction(
		// Params:
		&cp_amm.VestingParameters{
			CliffPoint:           cliffPoint,
			PeriodFrequency:      periodFrequency,
			CliffUnlockLiquidity: u128.GenUint128FromString(cliffUnlockLiquidity.String()),
			LiquidityPerPeriod:   u128.GenUint128FromString(liquidityPerPeriod.String()),
			NumberOfPeriod:       numberOfPeriod,
		},

		// Accounts:
		poolAddress,
		ownerPosition.Position,
		vesting,
		ownerPosition.PositionNftAccount,
		owner,
		payer,
		solana.SystemProgramID,
		eventAuthority,
		cp_amm.ProgramID,
	)

	if err != nil {
		return nil, err
	}
	return []solana.Instruction{lockIx}, nil
}

func (m *DammV2) LockPosition(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
	cliffPoint *uint64,
	periodFrequency uint64,
	cliffUnlockLiquidity *big.Int,
	liquidityPerPeriod *big.Int,
	numberOfPeriod uint16,
	vesting *solana.Wallet,
) (string, error) {

	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, poolState.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}

	userPosition = userPositions[0]
	instructions, err := LockPositionInstruction(
		ctx,
		payer.PublicKey(),
		owner.PublicKey(),
		userPosition,
		poolState.Address,
		cliffPoint,
		periodFrequency,
		cliffUnlockLiquidity,
		liquidityPerPeriod,
		numberOfPeriod,
		vesting.PublicKey(),
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
			case key.Equals(vesting.PublicKey()):
				return &vesting.PrivateKey
			case key.Equals(owner.PublicKey()):
				return &owner.PrivateKey
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

func PermanentLockPositionInstruction(
	ctx context.Context,
	owner solana.PublicKey,
	ownerPosition *UserPosition,
	poolAddress solana.PublicKey,
	permanentLockLiquidity *big.Int,
) ([]solana.Instruction, error) {

	lockIx, err := cp_amm.NewPermanentLockPositionInstruction(
		// Params:
		u128.GenUint128FromString(permanentLockLiquidity.String()),

		// Accounts:
		poolAddress,
		ownerPosition.Position,
		ownerPosition.PositionNftAccount,
		owner,
		eventAuthority,
		cp_amm.ProgramID,
	)

	if err != nil {
		return nil, err
	}
	return []solana.Instruction{lockIx}, nil
}

func (m *DammV2) PermanentLockPosition(
	ctx context.Context,
	owner *solana.Wallet,
	baseMint solana.PublicKey,
	permanentLockLiquidity *big.Int,
) (string, error) {

	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}
	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, poolState.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}
	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}
	userPosition = userPositions[0]

	instructions, err := PermanentLockPositionInstruction(
		ctx,
		owner.PublicKey(),
		userPosition,
		poolState.Address,
		permanentLockLiquidity,
	)
	if err != nil {
		return "", err
	}
	sig, err := solanago.SendTransaction(ctx,
		m.rpcClient,
		m.wsClient,
		instructions,
		owner.PublicKey(),
		func(key solana.PublicKey) *solana.PrivateKey {
			switch {
			case key.Equals(owner.PublicKey()):
				return &owner.PrivateKey
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

func SplitPositionInstruction(
	ctx context.Context,
	owner solana.PublicKey,
	ownerPosition *UserPosition,
	newOwner solana.PublicKey,
	newOwnerPositionNft solana.PublicKey,
	poolAddress solana.PublicKey,
	unlockedLiquidityPercentage uint8,
	permanentLockedLiquidityPercentage uint8,
	feeAPercentage uint8,
	feeBPercentage uint8,
	reward0Percentage uint8,
	reward1Percentage uint8,
) ([]solana.Instruction, error) {

	position, err := cp_amm.DerivePositionAddress(newOwnerPositionNft)
	if err != nil {
		return nil, err
	}

	positionNftAccount, err := cp_amm.DerivePositionNftAccount(newOwnerPositionNft)
	if err != nil {
		return nil, err
	}

	splitIx, err := cp_amm.NewSplitPositionInstruction(
		// Params:
		cp_amm.SplitPositionParameters{
			UnlockedLiquidityPercentage:        unlockedLiquidityPercentage,
			PermanentLockedLiquidityPercentage: permanentLockedLiquidityPercentage,
			FeeAPercentage:                     feeAPercentage,
			FeeBPercentage:                     feeBPercentage,
			Reward0Percentage:                  reward0Percentage,
			Reward1Percentage:                  reward1Percentage,
		},

		// Accounts:
		poolAddress,
		ownerPosition.Position,
		ownerPosition.PositionNftAccount,
		position,
		positionNftAccount,
		owner,
		newOwner,
		eventAuthority,
		cp_amm.ProgramID,
	)
	if err != nil {
		return nil, err
	}
	return []solana.Instruction{splitIx}, nil
}

func (m *DammV2) SplitPosition(
	ctx context.Context,
	payer *solana.Wallet,
	owner *solana.Wallet,
	newOwner *solana.Wallet,
	newOwnerPositionNft *solana.Wallet,
	baseMint solana.PublicKey,
	unlockedLiquidityPercentage uint8,
	permanentLockedLiquidityPercentage uint8,
	feeAPercentage uint8,
	feeBPercentage uint8,
	reward0Percentage uint8,
	reward1Percentage uint8,
) (string, error) {
	poolState, err := m.GetPoolByBaseMint(ctx, baseMint)
	if err != nil {
		return "", err
	}

	var userPosition *UserPosition
	userPositions, err := m.GetUserPositionByUserAndPoolPDA(ctx, poolState.Address, owner.PublicKey())
	if err != nil {
		return "", err
	}

	if len(userPositions) == 0 {
		return "", fmt.Errorf("no matching user_position")
	}

	userPosition = userPositions[0]

	instructions, err := SplitPositionInstruction(
		ctx,
		owner.PublicKey(),
		userPosition,
		newOwner.PublicKey(),
		newOwnerPositionNft.PublicKey(),
		poolState.Address,
		unlockedLiquidityPercentage,
		permanentLockedLiquidityPercentage,
		feeAPercentage,
		feeBPercentage,
		reward0Percentage,
		reward1Percentage,
	)
	if err != nil {
		return "", nil
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
			case key.Equals(owner.PublicKey()):
				return &owner.PrivateKey
			case key.Equals(newOwner.PublicKey()):
				return &newOwner.PrivateKey
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

func getPositionNftAccountsByUser(
	ctx context.Context,
	rpcClient *rpc.Client,
	user solana.PublicKey,
) (map[solana.PublicKey]*solanago.Account, error) {
	out, err := rpcClient.GetTokenAccountsByOwner(
		ctx,
		user,
		&rpc.GetTokenAccountsConfig{
			ProgramId: &solana.Token2022ProgramID,
		},
		&rpc.GetTokenAccountsOpts{
			Commitment: rpc.CommitmentFinalized,
			Encoding:   solana.EncodingBase64,
		},
	)
	if err != nil {
		return nil, err
	}

	data := make(map[solana.PublicKey]*solanago.Account)

	for _, keyedAcc := range out.Value {
		tokenAccount, err := new(solanago.AccountLayout).Decode(keyedAcc.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		tokenAccount.Address = keyedAcc.Pubkey

		if tokenAccount.Amount == 1 {
			position, err := cp_amm.DerivePositionAddress(tokenAccount.Mint)
			if err != nil {
				return nil, err
			}
			data[position] = tokenAccount
		}
	}
	return data, nil
}

func (m *DammV2) GetUserPositionByUserAndPoolPDA(
	ctx context.Context,
	poolAddress solana.PublicKey,
	user solana.PublicKey,
) ([]*UserPosition, error) {
	return GetUserPositionByUserAndPoolPDA(ctx, m.rpcClient, poolAddress, user)
}

func GetUserPositionByUserAndPoolPDA(
	ctx context.Context,
	rpcClient *rpc.Client,
	poolAddress solana.PublicKey,
	user solana.PublicKey,
) ([]*UserPosition, error) {
	// func (m *DammV2) GetUserPositionByBaseMint(ctx context.Context, cpammPool *Pool, user solana.PublicKey) ([]*UserPosition, error) {
	userPositionNftAccounts, err := getPositionNftAccountsByUser(ctx, rpcClient, user)
	if err != nil {
		return nil, err
	}

	if len(userPositionNftAccounts) == 0 {
		return nil, nil
	}

	positions, err := GetPositionsByPoolPDA(ctx, rpcClient, poolAddress)
	if err != nil {
		return nil, err
	}

	var positionResult []*UserPosition

	for _, v := range positions {
		account, ok := userPositionNftAccounts[v.Position]
		if !ok {
			continue
		}

		positionResult = append(positionResult, &UserPosition{
			Position:           v.Position,
			PositionState:      v.PositionState,
			PositionNftAccount: account.Address,
		})
	}

	return positionResult, nil
}

// fetchMultiplePositions
func fetchMultiplePositions(
	ctx context.Context,
	rpcClient *rpc.Client,
	addresses []solana.PublicKey,
) (map[solana.PublicKey]*cp_amm.Position, error) {
	accounts, err := solanago.GetMultipleAccountInfo(ctx, rpcClient, addresses)
	if err != nil {
		return nil, err
	}

	data := make(map[solana.PublicKey]*cp_amm.Position)
	for _, account := range accounts.Value {
		if account == nil {
			continue
		}
		obj, err := cp_amm.ParseAnyAccount(account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		position, ok := obj.(*cp_amm.Position)
		if !ok {
			return nil, fmt.Errorf("obj.(*cp_amm.Position) fail")
		}

		data[position.NftMint] = position
	}
	return data, nil
}

func (m *DammV2) GetUserPositionsByUser(
	ctx context.Context,
	user solana.PublicKey,
) ([]*UserPosition, error) {
	return GetUserPositionsByUser(ctx, m.rpcClient, user)
}

func GetUserPositionsByUser(
	ctx context.Context,
	rpcClient *rpc.Client,
	user solana.PublicKey,
) ([]*UserPosition, error) {
	userPositionNftAccounts, err := getPositionNftAccountsByUser(ctx, rpcClient, user)
	if err != nil {
		return nil, err
	}

	if len(userPositionNftAccounts) == 0 {
		return nil, nil
	}

	positionAddresses := slices.Collect(maps.Keys(userPositionNftAccounts))

	positionStates, err := fetchMultiplePositions(ctx, rpcClient, positionAddresses)
	if err != nil {
		return nil, err
	}

	var positionResult []*UserPosition
	for _, account := range userPositionNftAccounts {
		state, ok := positionStates[account.Mint]
		if !ok {
			continue
		}

		position, err := cp_amm.DerivePositionAddress(account.Mint)
		if err != nil {
			return nil, err
		}

		positionResult = append(positionResult, &UserPosition{
			PositionNftAccount: account.Address,
			Position:           position,
			PositionState:      state,
		})
	}
	return positionResult, nil
}

func (m *DammV2) GetPositionsByPoolPDA(
	ctx context.Context,
	poolAddress solana.PublicKey,
) ([]*Position, error) {
	return GetPositionsByPoolPDA(ctx, m.rpcClient, poolAddress)
}

func GetPositionsByPoolPDA(
	ctx context.Context,
	rpcClient *rpc.Client,
	poolAddress solana.PublicKey,
) ([]*Position, error) {

	opt := solanago.GenProgramAccountFilter(cp_amm.AccountKeyPosition, &solanago.Filter{
		Owner:  poolAddress,
		Offset: solanago.ComputeStructOffset(new(cp_amm.Position), "Pool"),
	})

	// opt := solanago.GenProgramAccountFilter(cp_amm.AccountKeyPosition, &solanago.Filter{Owner: poolAddress, Offset: 8})

	outs, err := rpcClient.GetProgramAccountsWithOpts(ctx, cp_amm.ProgramID, opt)
	if err != nil {
		if err == rpc.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}

	var list []*Position
	for _, out := range outs {
		obj, err := cp_amm.ParseAnyAccount(out.Account.Data.GetBinary())
		if err != nil {
			return nil, err
		}
		position, ok := obj.(*cp_amm.Position)
		if !ok {
			return nil, fmt.Errorf("obj.(*cp_amm.Position) fail")
		}

		list = append(list, &Position{
			Position:      out.Pubkey,
			PositionState: position,
		})
	}

	return list, nil
}

func canUnlockPosition(
	positionState *cp_amm.Position,
	vestings []*Vesting,
	currentPoint *big.Int,
) error {
	if len(vestings) == 0 {
		return nil
	}

	if positionState.PermanentLockedLiquidity.BigInt().Cmp(big.NewInt(0)) > 0 {
		return fmt.Errorf("Position is permanently locked")
	}

	for _, v := range vestings {
		vesting := v.VestingState
		if !cp_amm.IsVestingComplete(
			decimal.NewFromUint64(vesting.CliffPoint),
			decimal.NewFromUint64(vesting.PeriodFrequency),
			decimal.NewFromUint64(uint64(vesting.NumberOfPeriod)),
			decimal.NewFromBigInt(currentPoint, 0),
		) {
			return fmt.Errorf("Position has incomplete vesting schedule")
		}
	}
	return nil
}
