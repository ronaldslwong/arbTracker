package tradeLoop

import (
	"fmt"
	"sync"
	"time"
)

type BinMovement struct {
	Timestamp time.Time
	BinID     int
}

var binHistories = make(map[string][]BinMovement) // key: mint address
var mu sync.Mutex

func RecordBinMovement(mint string, binID int) {
	mu.Lock()
	defer mu.Unlock()

	now := time.Now()
	entry := BinMovement{Timestamp: now, BinID: binID}
	binHistories[mint] = append(binHistories[mint], entry)
	fmt.Println(binHistories[mint])
}

func pruneAndCalculateVolatility(mint string, windowSecs int) int {
	mu.Lock()
	defer mu.Unlock()

	history := binHistories[mint]
	cutoff := time.Now().Add(-time.Duration(windowSecs) * time.Second)

	var pruned []BinMovement
	var count int
	lastBin := -1

	for _, m := range history {
		if m.Timestamp.After(cutoff) {
			pruned = append(pruned, m)
			if m.BinID != lastBin {
				count++
				lastBin = m.BinID
			}
		}
	}

	binHistories[mint] = pruned
	return count
}
