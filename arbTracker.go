package main

import (
	"context"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"

	"arbTracker/configLoad"
	"arbTracker/fetcher"
	"arbTracker/globals"
	initialize "arbTracker/init"
	"arbTracker/tradeConfig"
	"arbTracker/tradeLoop"
	"arbTracker/types"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

var (
	grpcAddr           = flag.String("endpoint", "", "Solana gRPC address, in URI format e.g. https://api.rpcpool.com")
	token              = flag.String("x-token", "", "Token for authenticating")
	jsonInput          = flag.String("json", "", "JSON for subscription request, prefix with @ to read json from file")
	insecureConnection = flag.Bool("insecure", false, "Connect without TLS")
	slots              = flag.Bool("slots", false, "Subscribe to slots update")
	blocks             = flag.Bool("blocks", false, "Subscribe to block update")
	block_meta         = flag.Bool("blocks-meta", false, "Subscribe to block metadata update")
	signature          = flag.String("signature", "", "Subscribe to a specific transaction signature")
	resub              = flag.Uint("resub", 0, "Resubscribe to only slots after x updates, 0 disables this")

	accounts = flag.Bool("accounts", false, "Subscribe to accounts")

	transactions       = flag.Bool("transactions", false, "Subscribe to transactions, required for tx_account_include/tx_account_exclude and vote/failed.")
	voteTransactions   = flag.Bool("transactions-vote", false, "Include vote transactions")
	failedTransactions = flag.Bool("transactions-failed", false, "Include failed transactions")

	accountsFilter              arrayFlags
	accountOwnersFilter         arrayFlags
	transactionsAccountsInclude arrayFlags
	transactionsAccountsExclude arrayFlags
)

var kacp = keepalive.ClientParameters{
	Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
	Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
	PermitWithoutStream: true,             // send pings even without active streams
}

type MintStats struct {
	TokenCA      string      // Number of times seen
	TradeDetails []MintTrade // Total CU used
}

var mintTracker = make(map[string]*MintStats)
var mintTrackerMu sync.Mutex

type MintTrade struct {
	Timestamp time.Time
	CuPrice   float64
	Profit    float64
}

var mintTradeTracker = make(map[string]*MintTrade)
var mintTradeTrackerMu sync.Mutex

type DualWriter struct {
	console io.Writer
	file    io.Writer
}

func (dw *DualWriter) Write(p []byte) (n int, err error) {
	// Write to console
	n, err = dw.console.Write(p)
	if err != nil {
		return n, err
	}

	// Write to file
	_, err = dw.file.Write(p)
	return n, err
}

func printSignatures(sigs [][]byte) []string {
	var temp []string
	for _, sig := range sigs {
		b58 := base58.Encode(sig)
		temp = append(temp, b58)
	}
	return temp
}

func getTotalProfitFromTx(transaction *pb.SubscribeUpdate) uint64 {
	const wsolMint = "So11111111111111111111111111111111111111112"

	tx := transaction.GetTransaction().GetTransaction()

	if tx == nil || tx.Transaction == nil || tx.Meta == nil {
		return 0
	}

	accountKeys := tx.Transaction.Message.AccountKeys
	if len(accountKeys) == 0 {
		return 0
	}

	// Assume fee payer (first signer) is the arbitrage contract or bot wallet
	targetOwner := base58.Encode(accountKeys[0])

	var preSOL, postSOL uint64
	var preWSOL, postWSOL int64

	// Native SOL
	minLen := len(tx.Meta.PreBalances)
	if len(tx.Transaction.Message.AccountKeys) < minLen {
		minLen = len(tx.Transaction.Message.AccountKeys)
	}

	for i := 0; i < minLen; i++ {
		owner := base58.Encode(tx.Transaction.Message.AccountKeys[i])
		if owner == targetOwner {
			preSOL += tx.Meta.PreBalances[i]
			postSOL += tx.Meta.PostBalances[i]
		}
	}

	// Wrapped SOL token
	for _, b := range tx.Meta.PreTokenBalances {
		if b.Owner == targetOwner && b.Mint == wsolMint && b.UiTokenAmount != nil {
			if amt, err := strconv.ParseInt(b.UiTokenAmount.Amount, 10, 64); err == nil {
				preWSOL += amt
			}
		}
	}
	for _, b := range tx.Meta.PostTokenBalances {
		if b.Owner == targetOwner && b.Mint == wsolMint && b.UiTokenAmount != nil {
			if amt, err := strconv.ParseInt(b.UiTokenAmount.Amount, 10, 64); err == nil {
				postWSOL += amt
			}
		}
	}

	return (postSOL + uint64(postWSOL)) - (preSOL + uint64(preWSOL))
}

// getTradedToken returns the first non‑WSOL mint whose balance changed.
// Delta = post − pre.
func getTradedToken(preBalances, postBalances []*pb.TokenBalance) (mint string, delta int64, ok bool) {
	const wsolMint = "So11111111111111111111111111111111111111112"

	// Build quick lookup maps
	preMap := make(map[string]int64, len(preBalances))
	for _, b := range preBalances {
		if b.UiTokenAmount == nil {
			continue
		}
		if amt, err := strconv.ParseInt(b.UiTokenAmount.Amount, 10, 64); err == nil {
			preMap[b.Mint] += amt
		}
	}

	postMap := make(map[string]int64, len(postBalances))
	for _, b := range postBalances {
		if b.UiTokenAmount == nil {
			continue
		}
		if amt, err := strconv.ParseInt(b.UiTokenAmount.Amount, 10, 64); err == nil {
			postMap[b.Mint] += amt
		}
	}

	// Walk the postBalances slice in order — pick the first non‑WSOL whose balance changed
	for _, b := range postBalances {
		mint := b.Mint
		if mint == wsolMint {
			continue
		}
		preAmt := preMap[mint]
		postAmt := postMap[mint]
		if postAmt == preAmt {
			return mint, postAmt - preAmt, true
		}
	}

	return "", 0, false
}

func isArbitrageTx(preBalances, postBalances []*pb.TokenBalance, owner string) bool {
	const wsolMint = "So11111111111111111111111111111111111111112"

	// Map mint -> net change
	deltaMap := make(map[string]int64)

	// fmt.Println(preBalances, postBalances)

	for _, b := range preBalances {
		if b.Owner == owner {
			if b.UiTokenAmount == nil {
				continue
			}
			amt, err := strconv.ParseInt(b.UiTokenAmount.Amount, 10, 64)
			if err != nil {
				continue
			}
			deltaMap[b.Mint] -= amt
		}
	}

	for _, b := range postBalances {
		if b.Owner == owner {
			if b.UiTokenAmount == nil {
				continue
			}
			amt, err := strconv.ParseInt(b.UiTokenAmount.Amount, 10, 64)
			if err != nil {
				continue
			}
			deltaMap[b.Mint] += amt
		}
	}

	// Track WSOL gain separately
	wsolGain := deltaMap[wsolMint]
	delete(deltaMap, wsolMint)

	// Check if all other mints net to zero
	for _, delta := range deltaMap {
		if delta != 0 {
			return false
		}
	}

	return wsolGain > 0
}
func monitorTransactions(resp *pb.SubscribeUpdate, streamName string, config configLoad.Config) (*globals.TradedToken, bool) {
	var cuLimit, cuPrice uint64

	if accountData := resp.GetAccount(); accountData != nil {
		poolMkt, _ := solana.PublicKeyFromBase58(base58.Encode(accountData.GetAccount().Pubkey))
		activeBinId := int32(binary.LittleEndian.Uint32(accountData.Account.Data[76 : 76+4]))
		fetcher.UpdateDLMMActiveBin(poolMkt, activeBinId)
		return nil, false
	}

	if tx := resp.GetTransaction(); tx != nil {
		if streamName == "wallet" {
			tradeLoop.RecordLandedSlot(resp.GetTransaction().GetSlot(), config)
			// fmt.Println(tradeLoop.LandedSlots)

			return nil, false
		}

		if streamName == "main" {
			if isArbitrageTx(tx.GetTransaction().Meta.PreTokenBalances, tx.GetTransaction().Meta.PostTokenBalances, base58.Encode(tx.Transaction.Transaction.Message.AccountKeys[0])) {

				cuUsed := resp.GetTransaction().GetTransaction().Meta.GetComputeUnitsConsumed()

				for _, instr := range resp.GetTransaction().GetTransaction().Transaction.Message.Instructions {
					programIdIndex := instr.ProgramIdIndex
					programId := resp.GetTransaction().GetTransaction().Transaction.Message.AccountKeys[programIdIndex]

					if base58.Encode(programId) == "ComputeBudget111111111111111111111111111111" && len(instr.Data) > 0 {
						ixData := instr.Data
						switch ixData[0] {
						case 0x02:
							if len(ixData) >= 5 {
								cuLimit = uint64(ixData[1]) | uint64(ixData[2])<<8 | uint64(ixData[3])<<16 | uint64(ixData[4])<<24
							}
						case 0x03:
							if len(ixData) >= 9 {
								cuPrice = uint64(ixData[1]) | uint64(ixData[2])<<8 | uint64(ixData[3])<<16 | uint64(ixData[4])<<24 |
									uint64(ixData[5])<<32 | uint64(ixData[6])<<40 | uint64(ixData[7])<<48 | uint64(ixData[8])<<56
							}
						}
					}
				}

				mint, _, ok := getTradedToken(tx.GetTransaction().Meta.PreTokenBalances, tx.GetTransaction().Meta.PostTokenBalances)
				if ok {
					trades := &globals.TradedToken{
						Signature: printSignatures(resp.GetTransaction().Transaction.Transaction.Signatures)[0],
						CUUsed:    cuUsed,
						CULimit:   cuLimit,
						CUPrice:   float64(cuUsed) / 350000.0 * float64(cuPrice),
						Mint:      mint,
						Amount:    fmt.Sprint(getTotalProfitFromTx(resp)),
						Slot:      resp.GetTransaction().GetSlot(),
					}
					processTrade(*trades)
					return trades, true
				}
			}
		}
	}

	// Default fallback if no transaction / not arbitrage / no traded token found
	return nil, false
}

func processTrade(trade globals.TradedToken) {
	mintTrackerMu.Lock()
	defer mintTrackerMu.Unlock()

	stats, exists := mintTracker[trade.Mint]
	if !exists {
		stats = &MintStats{}
		mintTracker[trade.Mint] = stats
	}

	temp := MintTrade{
		Timestamp: time.Now(),
		CuPrice:   trade.CUPrice,
		Profit:    0, // You can replace this with parsed SOL profit if needed
	}
	stats.TradeDetails = append(stats.TradeDetails, temp)

	// Optional: update recent tracker
	mintTradeTrackerMu.Lock()
	mintTradeTracker[trade.Mint] = &temp
	mintTradeTrackerMu.Unlock()
}

func pruneOldTrades(config configLoad.Config) {
	mintTrackerMu.Lock()
	defer mintTrackerMu.Unlock()

	now := time.Now()
	for mint, stats := range mintTracker {
		var recentTrades []MintTrade
		for _, trade := range stats.TradeDetails {
			if now.Sub(trade.Timestamp) <= time.Duration(config.WindowSeconds)*time.Second {
				recentTrades = append(recentTrades, trade)
			}
		}

		if len(recentTrades) == 0 {
			delete(mintTracker, mint)
		} else {
			stats.TradeDetails = recentTrades
		}
	}
}

func startHotMintLogger(interval time.Duration, config configLoad.Config, ctx context.Context, dw *DualWriter) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			pruneOldTrades(config)
			printHotMints(config, ctx, dw)
			// fmt.Println(float64(len(globals.LandedSlots) / 150.0))
		}
	}()
}

func percentileCUPrice(prices []float64, percentile float64) float64 {
	if len(prices) == 0 {
		return 0
	}
	sort.Float64s(prices)
	index := int(float64(len(prices)-1) * percentile)
	return prices[index]
}
func extractCUPrices(trades []MintTrade) []float64 {
	var prices []float64
	for _, t := range trades {
		prices = append(prices, float64(t.CuPrice))
	}
	return prices
}
func printHotMints(config configLoad.Config, ctx context.Context, dw *DualWriter) {
	mintTrackerMu.Lock()
	defer mintTrackerMu.Unlock()

	fmt.Println("🔥 Hot Mints:")
	type sortable struct {
		mint  string
		stats *MintStats
	}
	var list []sortable
	for mint, stats := range mintTracker {
		list = append(list, sortable{mint, stats})
	}

	// Sort by number of trades in descending order
	sort.Slice(list, func(i, j int) bool {
		return len(list[i].stats.TradeDetails) > len(list[j].stats.TradeDetails)
	})

	types.HotMintsList = nil //clear hotmints to reload
	for i, item := range list {
		if i >= 2 {
			break
		}
		numTrades := len(item.stats.TradeDetails)
		CUPriceUse := percentileCUPrice(extractCUPrices(item.stats.TradeDetails), config.CuPricePercentile)
		if numTrades > config.NumArbsFilter && !globals.StringInList(item.mint, config.MintsIgnore) {
			fmt.Fprintf(dw, "[%s] Mint: %s, Trades: %d, 95 Percentile CU Price: %.0f\n",
				time.Now().Format("2006-01-02 15:04:05"), item.mint, len(item.stats.TradeDetails), CUPriceUse)
			types.HotMintsList = append(types.HotMintsList, types.HotMints{TokenCA: item.mint, CuPrice: CUPriceUse})
		}
	}
	if len(types.HotMintsList) > 0 {
		tradeConfig.PushToMaster(types.HotMintsList, config, ctx)

	} else {
		//set tradeconfigs back to 0
		types.TradeConfigs = []types.TradeConfig{}
		tradeLoop.TradeActive = false //reset dynamic CU flag
		tradeLoop.LastTradeTime = time.Now()

	}
}

func main() {
	// Open the file for writing (or create it if it doesn't exist)
	file, err := os.OpenFile("output.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Create a new DualWriter to write to both console and file
	dw := &DualWriter{
		console: os.Stdout,
		file:    file,
	}

	config := configLoad.LoadConfig("config.toml")

	// Local/Private RPC (best for speed and reliability)
	// Create a cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startHotMintLogger(time.Duration(config.CheckInterval)*time.Second, config, ctx, dw)

	initialize.Initialize(config, ctx)

	// set flags
	grpcAddr = &config.GRPCEndpoint
	transactions = new(bool)
	*transactions = true
	transactionsAccountsInclude = arrayFlags(config.AccountsMonitor)

	if *grpcAddr == "" {
		log.Fatalf("GRPC address is required. Please provide --endpoint parameter.")
	}

	u, err := url.Parse(*grpcAddr)
	if err != nil {
		log.Fatalf("Invalid GRPC address provided: %v", err)
	}

	// Infer insecure connection if http is given
	if u.Scheme == "http" {
		*insecureConnection = true
	}

	port := u.Port()
	if port == "" {
		if *insecureConnection {
			port = "80"
		} else {
			port = "443"
		}
	}
	hostname := u.Hostname()
	if hostname == "" {
		log.Fatalf("Please provide URL format endpoint e.g. http(s)://<endpoint>:<port>")
	}

	address := hostname + ":" + port

	// Handle termination signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("Received termination signal. Shutting down gracefully...")
		cancel()
	}()

	conn := grpc_connect(address, *insecureConnection)
	defer conn.Close()

	// Main stream for hot mints, only successful txs
	mainCtx, _ := context.WithCancel(ctx)
	grpc_subscribe_named(mainCtx, conn, "main", config.AccountsMonitor, globals.ToBoolPointer(false), config)

	// Wallet stream, allow failed txs too
	walletCtx, _ := context.WithCancel(ctx)
	grpc_subscribe_named(walletCtx, conn, "wallet", []string{config.TrackWallet}, globals.ToBoolPointer(true), config)

	// Instead of blocking with select {}, wait for the context to be canceled
	<-ctx.Done() // This will block until the cancel() is called

	log.Println("Gracefully shutting down.")
}

func grpc_connect(address string, plaintext bool) *grpc.ClientConn {
	var opts []grpc.DialOption
	if plaintext {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		pool, _ := x509.SystemCertPool()
		creds := credentials.NewClientTLSFromCert(pool, "")
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	opts = append(opts, grpc.WithKeepaliveParams(kacp))

	log.Println("Starting grpc client, connecting to", address)
	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	return conn
}

func grpc_subscribe_named(ctx context.Context, conn *grpc.ClientConn, streamName string, accountsToInclude []string, failedTxs *bool, config configLoad.Config) {
	globals.GlobalSubManager.Client = pb.NewGeyserClient(conn)

	// Initialize TradeChan for the streamName if not already initialized
	// Initialize StreamWorker for streamName if not already initialized
	if _, ok := globals.GlobalSubManager.StreamWorkers[streamName]; !ok {
		// Initialize a new StreamWorker with a TradeChan
		tradeChan := make(chan globals.TradedToken, 10000) // Adjust buffer size as needed
		worker := &globals.StreamWorker{
			Name:      streamName,
			Ctx:       ctx,
			Cancel:    func() {}, // Implement cancellation logic if needed
			Conn:      conn,
			TradeChan: tradeChan,
		}
		// Add StreamWorker to GlobalSubManager
		globals.GlobalSubManager.StreamWorkers[streamName] = worker
	}

	// Prepare subscription request
	subscription := &pb.SubscribeRequest{
		Transactions: map[string]*pb.SubscribeRequestFilterTransactions{
			streamName: {
				Failed:         failedTxs,
				Vote:           globals.ToBoolPointer(false),
				AccountInclude: accountsToInclude,
			},
		},
	}

	subscriptionJson, _ := json.Marshal(&subscription)
	log.Printf("[%s] Subscription JSON: %s", streamName, string(subscriptionJson))

	// Connect the stream
	stream, err := globals.GlobalSubManager.Client.Subscribe(ctx)
	if err != nil {
		log.Fatalf("[%s] Failed to start gRPC stream: %v", streamName, err)
	}

	// Link the stream to the worker
	worker := globals.GlobalSubManager.StreamWorkers[streamName]
	worker.Stream = stream // Assign the stream t

	err = stream.Send(subscription)
	if err != nil {
		log.Fatalf("[%s] Failed to send subscription request: %v", streamName, err)
	}

	worker.Subscription = subscription
	log.Printf("[%s] Subscription started: %+v", streamName, accountsToInclude)

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Printf("[%s] Context cancelled, closing stream...", streamName)
				return
			default:
				resp, err := stream.Recv()
				if err != nil {
					if err == io.EOF || ctx.Err() != nil {
						log.Printf("[%s] Stream closed cleanly", streamName)
					} else {
						log.Printf("[%s] Stream error: %v", streamName, err)
					}
					return
				}

				// Process received transaction
				trade, ok := monitorTransactions(resp, streamName, config)
				if !ok || trade == nil {
					continue
				}

				// Access the correct StreamWorker for this streamName
				worker := globals.GlobalSubManager.StreamWorkers[streamName]

				// Send trade to the correct stream's TradeChan
				select {
				case worker.TradeChan <- *trade:
					// fmt.Println(trade)
					// Successfully queued
				default:
					log.Printf("[%s] Trade channel is full! Dropping trade.", streamName)
				}
			}
		}
	}()
}
