package alt

import (
	blockhashrefresh "arbTracker/blockhashRefresh"
	"arbTracker/types"
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table" // Updated import
	"github.com/gagliardetto/solana-go/rpc"
)

const MAX_ALT_ADDRESSES_PER_EXTEND = 20

var mu sync.Mutex // separate package-level mutex if you want

// type ALTInfo struct {
// 	Address   solana.PublicKey                            // The address of the lookup table
// 	Addresses *addresslookuptable.AddressLookupTableState // The accounts stored in the lookup table
// }

// var altInfoList []ALTInfo

var AltMap = make(map[solana.PublicKey]*addresslookuptable.AddressLookupTableState)

func InitLoadAlt(ctx context.Context, client *rpc.Client, address string) {
	altAddress := solana.MustPublicKeyFromBase58(address)

	lookupTableAccount, err := addresslookuptable.GetAddressLookupTable(
		context.Background(),
		client,
		altAddress, // ALT public key you're using
	)
	if err != nil {
		fmt.Errorf("failed to fetch ALT: %w", err)
	}

	AltMap[altAddress] = lookupTableAccount

}

func AddNewAccountsToALT(altPubkey solana.PublicKey, payerPubkey solana.PublicKey, tradeConfig []types.TradeConfig) ([]solana.Instruction, error) {
	mu.Lock()
	defer mu.Unlock()

	// Fetch current ALT state
	altAccount := AltMap[altPubkey]
	if altAccount == nil || altAccount.Addresses == nil {
		return nil, fmt.Errorf("ALT not found or empty")
	}

	existing := make(map[string]struct{})
	for _, addr := range altAccount.Addresses {
		existing[addr.String()] = struct{}{}
	}

	// Gather all candidate accounts from TradeConfigs
	toAdd := make(map[string]solana.PublicKey)

	for _, cfg := range tradeConfig {
		// Wallet ATA
		toAdd[cfg.Ata.String()] = cfg.Ata

		for _, ray := range cfg.Raydium {
			for _, acc := range []solana.PublicKey{ray.ProgramId, ray.Authority, ray.AMM, ray.XVault, ray.SOLVault} {
				toAdd[acc.String()] = acc
			}
		}
		for _, cp := range cfg.RaydiumCP {
			for _, acc := range []solana.PublicKey{cp.ProgramId, cp.Authority, cp.Pool, cp.AMMConfig, cp.XVault, cp.SOLVault, cp.Observation} {
				toAdd[acc.String()] = acc
			}
		}
		for _, pump := range cfg.Pump {
			for _, acc := range []solana.PublicKey{pump.Pool, pump.XAccount, pump.SOLAccount, pump.FeeTokenWallet} {
				toAdd[acc.String()] = acc
			}
		}
		for _, dlmm := range cfg.DLMM {
			for _, acc := range []solana.PublicKey{dlmm.Pair, dlmm.XVault, dlmm.SOLVault, dlmm.Oracle} {
				toAdd[acc.String()] = acc
			}
			for _, bin := range dlmm.BinArrays {
				toAdd[bin.PublicKey.String()] = bin.PublicKey
			}
		}
	}

	// Remove already existing addresses
	var newAccounts []solana.PublicKey
	for k, v := range toAdd {
		if _, exists := existing[k]; !exists {
			newAccounts = append(newAccounts, v)
		}
	}

	if len(newAccounts) == 0 {
		return nil, nil // Nothing to add
	}

	// Build ALT extend instruction
	addInstr, _ := BuildExtendALTInstruction(altPubkey, payerPubkey, payerPubkey, newAccounts)

	//update alt struct to include new acocunts
	// AltMap[altPubkey].Addresses = append(AltMap[altPubkey].Addresses, newAccounts...)

	return []solana.Instruction{addInstr}, nil
}

// BuildExtendALTInstruction builds a single ExtendLookupTable instruction
func BuildExtendALTInstruction(
	lookupTable solana.PublicKey,
	authority solana.PublicKey,
	payer solana.PublicKey,
	newAddresses []solana.PublicKey,
) (solana.Instruction, error) {
	// newAddresses = []solana.PublicKey{(solana.MustPublicKeyFromBase58("5CXM9EPQv25y5aQky8Qyt2fRmKVNWuby9PQVCDaXrnZF")), (solana.MustPublicKeyFromBase58("4cENZePVsZAHkVwopddYCJhgRgjy1HLuffyhDyprUaSc"))}
	newAddresses = DeduplicateUpload(newAddresses)
	// Create instruction data matching the exact byte pattern from JS
	data := []byte{
		2,       // Discriminator byte
		0, 0, 0, // First 4 bytes of metadata (all zeros)
	}

	// Little-endian encoded u64 count of addresses
	// addressCount := uint64(len(newAddresses))
	// countBytes := make([]byte, 8)
	// binary.LittleEndian.PutUint64(countBytes, addressCount)
	// data = append(data, countBytes...)

	count := uint64(len(newAddresses) - 1)
	// count := uint64(3)
	fmt.Println(len(newAddresses))
	countBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(countBytes, count)

	// data = append(data, offsetBytes...)
	data = append(data, countBytes...)

	// Add all addresses
	for _, addr := range newAddresses {
		if addr.String() != "11111111111111111111111111111111" {
			data = append(data, addr[:]...)
			println(addr.String())
		}
	}

	fmt.Printf("Instruction Data (hex): %x\n", data)
	// fmt.Println(newAddresses)
	programID := solana.MustPublicKeyFromBase58("AddressLookupTab1e1111111111111111111111111")

	return &solana.GenericInstruction{
		AccountValues: []*solana.AccountMeta{
			{PublicKey: lookupTable, IsSigner: false, IsWritable: true},
			{PublicKey: authority, IsSigner: true, IsWritable: false},
			{PublicKey: payer, IsSigner: true, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		ProgID:    programID,
		DataBytes: data,
	}, nil
}

func SendExtendALTTransaction(
	client *rpc.Client,
	altPubkey solana.PublicKey,
	payer *solana.Wallet, // Wallet object with private key
	tradeConfig []types.TradeConfig,
) (solana.Signature, error) {
	ctx := context.Background()

	// Step 1: Build the extend instructions
	instrs, err := AddNewAccountsToALT(altPubkey, payer.PublicKey(), tradeConfig)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to prepare ALT instructions: %w", err)
	}
	if len(instrs) == 0 {
		return solana.Signature{}, fmt.Errorf("no new addresses to add to ALT")
	}

	// Step 3: Build transaction
	tx, err := solana.NewTransaction(
		instrs,
		blockhashrefresh.GetCachedBlockhash(),
		solana.TransactionPayer(payer.PublicKey()),
	)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to build transaction: %w", err)
	}

	// Step 4: Sign transaction
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if payer.PublicKey().Equals(key) {
			return &payer.PrivateKey
		}
		return nil
	})
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to sign transaction: %w", err)
	}
	fmt.Println("signed")

	// Step 5: Send transaction
	sig, err := client.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       true,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	fmt.Println("ALT Extend transaction sent:", sig.String())
	return sig, nil
}

// func GetALTMap(instruction solana.Instruction,
// 	altMap map[solana.PublicKey]*addresslookuptable.AddressLookupTableState,
// ) map[solana.PublicKey]solana.PublicKeySlice { // Return map type
// 	altMapResult := make(map[solana.PublicKey]solana.PublicKeySlice)

// 	dedupeInstruction := DeduplicateAccounts(instruction.Accounts())
// 	accountMatched := make(map[string]bool)

// 	for _, acct := range dedupeInstruction {
// 		acctPubKeyStr := acct.PublicKey.String() // Or however you get string representation

// 		// Skip if this account is already matched to an ALT
// 		if accountMatched[acctPubKeyStr] {
// 			continue
// 		}

// 		fmt.Println("instruction address", acct)

// 		for altPubKey, altState := range altMap {
// 			for _, key := range altState.Addresses {
// 				if acct.PublicKey.Equals(key) {
// 					altMapResult[altPubKey] = append(altMapResult[altPubKey], key)
// 					accountMatched[acctPubKeyStr] = true
// 					break // Exit once a match is found
// 				}
// 			}
// 			// If account was matched to this ALT, no need to check other ALTs
// 			if accountMatched[acctPubKeyStr] {
// 				break
// 			}
// 		}
// 	}

// 	return altMapResult
// }

func DeduplicateAccounts(accounts []*solana.AccountMeta) []*solana.AccountMeta {
	seen := make(map[string]bool)
	result := make([]*solana.AccountMeta, 0, len(accounts))

	for _, acc := range accounts {
		pubkeyStr := acc.PublicKey.String() // Or however you get the account address as string
		if !seen[pubkeyStr] {
			seen[pubkeyStr] = true
			result = append(result, acc)
		}
	}

	return result
}

func DeduplicateUpload(accounts []solana.PublicKey) []solana.PublicKey {
	seen := make(map[string]bool)
	result := make([]solana.PublicKey, 0, len(accounts))

	for _, acc := range accounts {
		pubkeyStr := acc.String() // Or however you get the account address as string
		if !seen[pubkeyStr] {
			seen[pubkeyStr] = true
			result = append(result, acc)
		}
	}

	return result
}

func GetALTMap(
	instructions []solana.Instruction,
	altMap map[solana.PublicKey]*addresslookuptable.AddressLookupTableState,
) []solana.MessageAddressTableLookup {
	// Create the result slice
	var lookups []solana.MessageAddressTableLookup
	// Process each ALT
	for tableKey, tableState := range altMap {
		var writableIndexes, readonlyIndexes []uint8

		// Use maps to track unique indexes and avoid duplicates
		writableIndexMap := make(map[uint8]bool)
		readonlyIndexMap := make(map[uint8]bool)

		// For each address in the ALT, check if it's used in the instructions
		for i, altAddress := range tableState.Addresses {
			// fmt.Println("table length", len(tableState.Addresses))
			fmt.Println("table entry", altAddress.String())

			for _, ix := range instructions {
				for _, acct := range ix.Accounts() {
					if acct.PublicKey.Equals(altAddress) {
						// Convert to uint8 for the index
						index := uint8(i)

						// Add to the appropriate map if not already there
						if acct.IsWritable {
							writableIndexMap[index] = true
						} else {
							readonlyIndexMap[index] = true
						}

						// We've found a match for this address, no need to check other accounts
						break
					}
				}
			}
		}

		// Convert maps to slices
		for idx := range writableIndexMap {
			writableIndexes = append(writableIndexes, idx)
		}

		for idx := range readonlyIndexMap {
			readonlyIndexes = append(readonlyIndexes, idx)
		}

		// Only add the ALT lookup if it's actually being used
		if len(writableIndexes)+len(readonlyIndexes) > 0 {
			lookups = append(lookups, solana.MessageAddressTableLookup{
				AccountKey:      tableKey,
				WritableIndexes: writableIndexes,
				ReadonlyIndexes: readonlyIndexes,
			})
		}
	}

	return lookups
}
