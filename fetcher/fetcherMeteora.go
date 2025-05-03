package fetcher

import (
	"arbTracker/configLoad"
	"arbTracker/globals"
	"arbTracker/types"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"

	"github.com/gagliardetto/solana-go"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

func FloorDiv(a, b int32) int32 {
	if (a < 0) != (b < 0) && a%b != 0 {
		return a/b - 1
	}
	fmt.Println(a / b)
	return a / b
}

func GetOracle(pool string) solana.PublicKey {
	oracle, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("oracle"),
			solana.MustPublicKeyFromBase58(pool).Bytes(),
		},
		solana.MustPublicKeyFromBase58("LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo"),
	)
	if err != nil {
		log.Fatalf("Failed to derive oracle PDA: %v", err)
	}
	return oracle
}

func DecodeActiveBinID(poolPubkey solana.PublicKey) (int32, error) {

	accountInfo, err := globals.RPCClient.GetAccountInfo(context.Background(), poolPubkey)
	if err != nil {
		log.Fatalf("failed to fetch account: %v", err)
	}

	data := accountInfo.Value.Data.GetBinary()
	if data == nil {
		log.Fatalf("data is nil or not binary-encoded")
	}

	// The active_bin_id is assumed to be located at a specific byte offset.
	// Adjust this offset according to your actual data layout in the pool account.
	const activeBinIDOffset = 76 // Replace with the correct byte offset for the active_bin_id
	if len(data) < int(activeBinIDOffset+4) {
		return 0, fmt.Errorf("account data is too short, expected at least %d bytes", activeBinIDOffset+4)
	}

	// Extract the active_bin_id (4 bytes)
	activeBinID := int32(binary.LittleEndian.Uint32(data[activeBinIDOffset : activeBinIDOffset+4]))
	return activeBinID, nil
}

func DeriveBinArrayPDA(lbPair solana.PublicKey, binArrayIndex int64) (solana.PublicKey, uint8, error) {
	// Signed i64 little-endian
	binIndexBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(binIndexBytes, uint64(binArrayIndex))
	if binArrayIndex < 0 {
		binary.LittleEndian.PutUint64(binIndexBytes, uint64(uint64(int64(binArrayIndex))))
	}

	seed := [][]byte{
		[]byte("bin_array"),
		lbPair.Bytes(),
		binIndexBytes,
	}

	programID := solana.MustPublicKeyFromBase58("LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo")
	return solana.FindProgramAddress(seed, programID)
}

func GetSurroundingBinPDAs(activeID int32, lbPair solana.PublicKey) ([]solana.AccountMeta, error) {
	// Define the range of bins to scan (±25 bins)
	// startBin := activeID - maxBins/2
	// endBin := activeID + maxBins/2

	// Calculate the bin array IDs
	curArrayID := FloorDiv(activeID, 64)
	startArrayID := curArrayID - 1
	endArrayID := curArrayID + 1
	// fmt.Println("start ", startArrayID)
	// fmt.Println("end ", endArrayID)

	// Prepare a slice to store the AccountMetas
	metas := make([]solana.AccountMeta, 0, endArrayID-startArrayID+1)

	for i := startArrayID; i <= endArrayID; i++ {
		pda, _, err := DeriveBinArrayPDA(lbPair, int64(i))
		if err != nil {
			return nil, err
		}

		// Adjust isSigner and isWritable depending on your program's expectations
		// meta := solana.NewAccountMeta(pda, false, false)
		metas = append(metas, solana.AccountMeta{
			PublicKey:  pda,
			IsSigner:   false,
			IsWritable: true,
		})
	}

	return metas, nil

}

func FetchMeteoraDLMM(tokenCA string, mktList []string, config configLoad.Config, ctx context.Context) []types.DLMMTriple {
	var returnMeteora []types.DLMMTriple

	for numMkts, x := range mktList {
		if numMkts < config.MeteoraMkts {
			pool, xAccount, sOLAccount, _ := getPoolTokens(x, tokenCA, config)
			fmt.Println(pool)
			tempPool := solana.MustPublicKeyFromBase58(pool)
			binIds, err := DecodeActiveBinID(tempPool)
			fmt.Println("active id: ", binIds)
			binArrays, err := GetSurroundingBinPDAs(binIds, tempPool) //, int32(config.MaxBins))
			fmt.Println(binArrays)
			if err != nil {
				log.Printf("error getting bin arrays: %v", err)
				return []types.DLMMTriple{}
			}
			tempPump := types.DLMMTriple{
				DlmmProgramId:      solana.MustPublicKeyFromBase58("LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo"),
				DlmmEventAuthority: solana.MustPublicKeyFromBase58("D1ZN9Wj1fRSUQfCjhvnu1hqDMT7hzjzBBpi12nVniYD6"),
				Pair:               tempPool,
				XVault:             solana.MustPublicKeyFromBase58(xAccount),
				SOLVault:           solana.MustPublicKeyFromBase58(sOLAccount),
				Oracle:             GetOracle(pool),
				BinID:              binIds,
				BinArrays:          binArrays,
			}
			returnMeteora = append(returnMeteora, tempPump)
		}

	}

	//subscribe to new bin feed

	return returnMeteora
}
func UpdateDLMMActiveBin(market solana.PublicKey, activeID int32) error {
	types.Mu.Lock()
	defer types.Mu.Unlock()

	for i := range types.TradeConfigs {
		cfg := &types.TradeConfigs[i]
		for j := range cfg.DLMM {
			triple := &cfg.DLMM[j]
			if triple.Pair == market && triple.BinID != activeID {
				// You already have cfg.Mint here for PDA derivation
				newPDA, err := GetSurroundingBinPDAs(activeID, triple.Pair)
				if err != nil {
					return err
				}

				triple.BinID = int32(activeID)
				triple.BinArrays = newPDA
				fmt.Println("Updated bin for ", cfg.Mint, triple.BinID)
			}
		}
	}

	return nil
}

// /////////////////
func ReplaceDLMMAccountSubscriptions(workerName string, pda []solana.PublicKey) {
	// Step 1: Resubscribe with an empty account list (unsubscribe from the old accounts)

	globals.GlobalSubManager.SendMutex.Lock()
	defer globals.GlobalSubManager.SendMutex.Unlock()
	for _, key := range pda {
		keyStr := key.String()
		if globals.GlobalSubManager.CurrentDLMMPools[keyStr] {
			// Already subscribed
			return
		}
		globals.GlobalSubManager.CurrentDLMMPools[keyStr] = true

		log.Printf("[GRPC] Subscribing to new bin PDA (resending full subscription): %s", keyStr)
	}

	err := ResubscribeEmptyAccountStream(workerName)
	if err != nil {
		fmt.Errorf("[GRPC] Error during resubscribe (empty): %v", err)
	}
	// Step 2: Resubscribe with the updated account list (subscribe to new accounts)
	err = ResubscribeWithUpdatedAccounts(workerName)
	if err != nil {
		fmt.Errorf("[GRPC] Error during resubscribe (updated accounts): %v", err)
	}

}

func ResubscribeEmptyAccountStream(workerName string) error {
	worker, exists := globals.GlobalSubManager.StreamWorkers[workerName]
	if !exists {
		return fmt.Errorf("[GRPC] No StreamWorker found for %s", workerName)
	}

	if worker.Subscription == nil {
		return fmt.Errorf("[GRPC] No active subscription for %s", workerName)
	}

	oldSubscription := worker.Subscription // Get the current subscription object

	// Create a new subscribe request with no accounts
	emptyRequest := &pb.SubscribeRequest{
		Accounts: nil, // No accounts to unsubscribe from
	}
	emptyRequest.Transactions = oldSubscription.Transactions // Reuse the original transaction request

	// Send the empty subscription request to effectively unsubscribe
	err := worker.Stream.Send(emptyRequest)
	if err != nil {
		return fmt.Errorf("[GRPC] Failed to unsubscribe from stream %s: %v", workerName, err)
	}

	// Reset the subscription in the worker
	// worker.Subscription = nil
	log.Printf("[GRPC] Successfully unsubscribed (cleared accounts) for %s", workerName)
	return nil
}

func ResubscribeWithUpdatedAccounts(workerName string) error {
	worker, exists := globals.GlobalSubManager.StreamWorkers[workerName]
	if !exists {
		return fmt.Errorf("[GRPC] No StreamWorker found for %s", workerName)
	}

	var allPDAs []string
	for key := range globals.GlobalSubManager.CurrentDLMMPools {
		allPDAs = append(allPDAs, key)
	}
	if len(allPDAs) == 0 {
		log.Println("Warning: No DLMMPools found to subscribe.")
	}

	oldSubscription := worker.Subscription // Get the current subscription object

	// Create the updated subscribe request with the new accounts
	updatedRequest := &pb.SubscribeRequest{
		Accounts: map[string]*pb.SubscribeRequestFilterAccounts{
			"dlmm_bins": {
				Account: allPDAs, // New account list
			},
		},
	}
	updatedRequest.Transactions = oldSubscription.Transactions // Reuse the original transaction request

	// Resend the subscription with the new accounts
	err := worker.Stream.Send(updatedRequest)
	if err != nil {
		return fmt.Errorf("[GRPC] Failed to resubscribe with updated accounts for %s: %v", workerName, err)
	}

	// Update the subscription in the worker
	if worker.Subscription == nil {
		worker.Subscription = &pb.SubscribeRequest{}
	}

	// You can add more logic here if necessary (e.g., transactions, other fields)
	worker.Subscription.Accounts = updatedRequest.Accounts
	log.Printf("[GRPC] Successfully resubscribed with updated accounts for %s", workerName)

	subscriptionJson, _ := json.Marshal(&updatedRequest)
	log.Printf("[%s] Subscription JSON: %s", "main", string(subscriptionJson))
	return nil
}
