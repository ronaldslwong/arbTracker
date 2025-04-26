package types

import (
	"sync"

	"github.com/gagliardetto/solana-go"
)

var (
	LatestBlockhash solana.Hash
	BlockhashMutex  sync.RWMutex
)

type HotMints struct {
	TokenCA string
	CuPrice float64
}

var HotMintsList []HotMints

// Structs (you already have these)
type RaydiumPool struct {
	ProgramId solana.PublicKey
	Authority solana.PublicKey
	AMM       solana.PublicKey
	XVault    solana.PublicKey
	SOLVault  solana.PublicKey
}

type RaydiumCPPool struct {
	ProgramId   solana.PublicKey
	Authority   solana.PublicKey
	Pool        solana.PublicKey
	AMMConfig   solana.PublicKey
	XVault      solana.PublicKey
	SOLVault    solana.PublicKey
	Observation solana.PublicKey
}

type PumpPool struct {
	ProgramId      solana.PublicKey
	GlobalConfig   solana.PublicKey
	Authority      solana.PublicKey
	FeeWallet      solana.PublicKey
	Pool           solana.PublicKey
	XAccount       solana.PublicKey
	SOLAccount     solana.PublicKey
	FeeTokenWallet solana.PublicKey
}

type DLMMTriple struct {
	DlmmProgramId      solana.PublicKey // DLMM program ID
	DlmmEventAuthority solana.PublicKey // DLMM event authority
	Pair               solana.PublicKey
	XVault             solana.PublicKey
	SOLVault           solana.PublicKey
	Oracle             solana.PublicKey
	BinID              int32
	BinArrays          []solana.AccountMeta
}

type TradeConfig struct {
	Mint      string
	CA        solana.PublicKey
	CUPrice   uint64
	CULimit   uint64
	Ata       solana.PublicKey
	Raydium   []RaydiumPool
	RaydiumCP []RaydiumCPPool
	Pump      []PumpPool
	DLMM      []DLMMTriple
}

// Exported map and lock
var (
	TradeConfigs = []TradeConfig{}
	Mu           sync.Mutex
)

func GetAllConfigs() []TradeConfig {
	Mu.Lock()
	defer Mu.Unlock()

	// Return a shallow copy of the slice to avoid race conditions
	copy := make([]TradeConfig, len(TradeConfigs))
	copy = append(copy[:0], TradeConfigs...) // efficient copy
	return copy
}
