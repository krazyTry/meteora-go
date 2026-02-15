package helpers

import (
	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func PositionByPoolFilter(pool solanago.PublicKey) rpc.RPCFilter {
	return rpc.RPCFilter{Memcmp: &rpc.RPCFilterMemcmp{Offset: 8, Bytes: solanago.Base58(pool.Bytes())}}
}

func VestingByPositionFilter(position solanago.PublicKey) rpc.RPCFilter {
	return rpc.RPCFilter{Memcmp: &rpc.RPCFilterMemcmp{Offset: 8, Bytes: solanago.Base58(position.Bytes())}}
}

func OffsetBasedFilter(value solanago.PublicKey, offset uint64) []rpc.RPCFilter {
	return []rpc.RPCFilter{{Memcmp: &rpc.RPCFilterMemcmp{Offset: offset, Bytes: solanago.Base58(value.Bytes())}}}
}
