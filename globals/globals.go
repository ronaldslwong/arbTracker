package globals

import (
	"context"
	"fmt"
	"sync"

	// "arbTracker/configLoad"

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
	// GlobalSubManager *SubscriptionManager
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

// type SubscriptionManager struct {
// 	Stream    pb.Geyser_SubscribeClient
// 	Context   context.Context
// 	Cancel    context.CancelFunc
// 	TradeChan chan TradedToken
// 	Config    configLoad.Config
// 	SendMutex sync.Mutex
// 	// Optional: track current subscribed PDAs
// 	CurrentDLMMPools map[string]bool

//		// Add these fields
//		EnableTransactions  bool
//		FailedTransactions  *bool
//		VoteTransactions    *bool
//		TransactionsInclude []string
//		TransactionsExclude []string
//	}
type StreamWorker struct {
	Name         string
	Ctx          context.Context
	Cancel       context.CancelFunc
	Conn         *grpc.ClientConn
	Stream       pb.Geyser_SubscribeClient
	TradeChan    chan TradedToken
	Subscription *pb.SubscribeRequest
}

// GlobalSubManager holds the workers for each stream.
type GlobalSubManagerType struct {
	StreamWorkers    map[string]*StreamWorker
	Client           pb.GeyserClient // Add the gRPC client field here
	SendMutex        sync.Mutex
	CurrentDLMMPools map[string]bool
	// New field you need to add:
	Workers map[string]*StreamWorker
}

// Initialize the GlobalSubManager if needed
var GlobalSubManager = &GlobalSubManagerType{
	StreamWorkers:    make(map[string]*StreamWorker),
	CurrentDLMMPools: make(map[string]bool),
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

// CreateWorker function for creating a new worker with a stream
func (gsm *GlobalSubManagerType) CreateWorker(name string) (*StreamWorker, error) {
	// Ensure gsm.Client is initialized
	if gsm.Client == nil {
		return nil, fmt.Errorf("gRPC client is not initialized")
	}

	// Create the subscription stream
	stream, err := gsm.Client.Subscribe(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	// Create and return the worker with the stream
	worker := &StreamWorker{
		Name:         name,
		Stream:       stream,
		Subscription: nil,
	}

	return worker, nil
}
