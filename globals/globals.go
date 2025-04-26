package globals

import (
	"context"
	"sync"

	"arbTracker/configLoad"

	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	"github.com/gagliardetto/solana-go/rpc"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"google.golang.org/grpc"
)

var (
	RPCClient     *rpc.Client
	Blockhash     string
	TradeInterval int
	PrivateKey    *solana.PrivateKey
	AltAddress    *addresslookuptable.AddressLookupTableState
	AltPubKey     solana.PublicKey
	// Logger        *log.Logger
	// add more global variables as needed
	SubscribedBinPDAs = struct {
		sync.RWMutex
		Pdas map[string]bool
	}{Pdas: make(map[string]bool)}
	GlobalSubManager *SubscriptionManager
)

type TradedToken struct {
	Signature string
	CUUsed    uint64
	CULimit   uint64
	CUPrice   float64
	Mint      string
	Amount    string // string to avoid big int issues
	Slot      uint64
	Timestamp int64 // unix timestamp, if available
}

type SubscriptionManager struct {
	Stream    pb.Geyser_SubscribeClient
	Context   context.Context
	Cancel    context.CancelFunc
	TradeChan chan TradedToken
	Config    configLoad.Config
	SendMutex sync.Mutex
	// Optional: track current subscribed PDAs
	CurrentDLMMPools map[string]bool

	// Add these fields
	EnableTransactions  bool
	FailedTransactions  *bool
	VoteTransactions    *bool
	TransactionsInclude []string
	TransactionsExclude []string
}
type StreamWorker struct {
	Name         string
	Ctx          context.Context
	Cancel       context.CancelFunc
	Conn         *grpc.ClientConn
	Stream       pb.Geyser_SubscribeClient
	TradeChan    chan TradedToken
	Subscription *pb.SubscribeRequest
}

func ToBoolPointer(b bool) *bool {
	return &b
}
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func StringInList(str string, list []string) bool {
	lookup := make(map[string]struct{})
	for _, item := range list {
		lookup[item] = struct{}{}
	}
	_, exists := lookup[str]
	return exists
}

var LandedSlots = make(map[uint64]struct{})
var LandedMu sync.Mutex
var CuBlock = 1

// Call this on each landed tx from your wallet
func RecordLandedSlot(slot uint64) {
	LandedMu.Lock()
	defer LandedMu.Unlock()

	LandedSlots[slot] = struct{}{}
	cutoff := slot - 150 // last 150 slots only

	// Clean up old entries
	for s := range LandedSlots {
		if s < cutoff {
			delete(LandedSlots, s)
		}
	}
}
