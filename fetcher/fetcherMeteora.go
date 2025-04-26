package fetcher

import (
	"arbTracker/configLoad"
	"arbTracker/globals"
	"arbTracker/types"
	"context"
	"encoding/binary"
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
	fmt.Println("start ", startArrayID)
	fmt.Println("end ", endArrayID)

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

	for _, x := range mktList {
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

	//subscribe to new bin feed

	return returnMeteora
}
func UnsubscribeBinPDA(pda solana.PublicKey) {
	key := pda.String()

	globals.GlobalSubManager.SendMutex.Lock()
	defer globals.GlobalSubManager.SendMutex.Unlock()

	if _, exists := globals.GlobalSubManager.CurrentDLMMPools[key]; !exists {
		return // Not subscribed
	}

	delete(globals.GlobalSubManager.CurrentDLMMPools, key)
	log.Printf("[GRPC] Unsubscribing (resending full subscription without): %s", key)

	// Re-send updated SubscribeRequest with all remaining PDAs
	resendFullBinPDASubscription()
}

func resendFullBinPDASubscription() {
	var allPDAs []string
	for key := range globals.GlobalSubManager.CurrentDLMMPools {
		allPDAs = append(allPDAs, key)
	}
	if len(allPDAs) == 0 {
		log.Println("Warning: No DLMMPools found to subscribe.")
	}

	req := &pb.SubscribeRequest{}

	if len(allPDAs) == 0 {
		log.Println("Warning: No DLMMPools found to subscribe. Setting Accounts filter to nil.")
		req.Accounts = nil
	} else {
		req.Accounts = map[string]*pb.SubscribeRequestFilterAccounts{
			"dlmm_bins": {
				Account: allPDAs,
			},
		}
	}

	// Ensure transactions are included if the flag is set
	if globals.GlobalSubManager.EnableTransactions {
		req.Transactions = map[string]*pb.SubscribeRequestFilterTransactions{
			"transactions_sub": {
				Failed:         globals.GlobalSubManager.FailedTransactions,
				Vote:           globals.GlobalSubManager.VoteTransactions,
				AccountInclude: globals.GlobalSubManager.TransactionsInclude,
				AccountExclude: globals.GlobalSubManager.TransactionsExclude,
			},
		}
	}
	// Debugging: Check and log the Transactions part of the request
	if req.Transactions != nil {
		log.Printf("Transactions part of the subscription: %+v", req)
	} else {
		log.Println("Transactions part is nil or dropped!")
	}

	// Check if the stream is active
	if globals.GlobalSubManager.Stream == nil {
		log.Println("Stream is not initialized.")
		return
	}

	// Send the updated subscription request
	err := globals.GlobalSubManager.Stream.Send(req)
	if err != nil {
		log.Printf("Error resending full bin PDA subscription: %v", err)
		return
	}

	log.Println("Successfully resent full bin PDA subscription.")
}

func SubscribeBinPDA(pda solana.PublicKey) {
	key := pda.String()

	globals.GlobalSubManager.SendMutex.Lock()
	defer globals.GlobalSubManager.SendMutex.Unlock()

	if globals.GlobalSubManager.CurrentDLMMPools[key] {
		// Already subscribed
		return
	}

	globals.GlobalSubManager.CurrentDLMMPools[key] = true
	log.Printf("[GRPC] Subscribing to new bin PDA (resending full subscription): %s", key)

	// Re-send full subscription including the new PDA
	resendFullBinPDASubscription()
}

func ReplaceDLMMAccountSubscriptions(poolAccounts []solana.PublicKey) {
	// Unsubscribe from all current pool subscriptions
	for key := range globals.GlobalSubManager.CurrentDLMMPools {
		pda, _ := solana.PublicKeyFromBase58(key)
		UnsubscribeBinPDA(pda) // consider renaming this to UnsubscribeAccount or UnsubscribeDLMMAccount
	}

	// Subscribe to new pool accounts
	for _, pool := range poolAccounts {
		SubscribeBinPDA(pool) // consider renaming to SubscribeDLMMAccount
	}
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
