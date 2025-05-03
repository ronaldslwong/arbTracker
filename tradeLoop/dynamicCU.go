package tradeLoop

import (
	"log"
	"sync"
	"time"

	"arbTracker/configLoad"
)

var LandedSlots = make(map[uint64]struct{})
var LandedMu sync.Mutex
var CuBlock = 1

// // trade tracking
var CurrentMultiplier float64 = 1.0 // Starting multiplier
var dynamicCUActive bool = false    // Flag to indicate if tracking is active
var TradeActive bool = false        // To track if any trades are active
var LastTradeTime time.Time         // Track the last trade time for turning tracking on/off

// Call this on each landed tx from your wallet
func RecordLandedSlot(slot uint64, config configLoad.Config) {
	LandedMu.Lock()
	defer LandedMu.Unlock()

	LandedSlots[slot] = struct{}{}
	cutoff := slot - uint64(config.SlotsToCheck) // last 150 slots only

	// Clean up old entries
	for s := range LandedSlots {
		if s < cutoff {
			delete(LandedSlots, s)
		}
	}
}

func UpdateTrackingState() {
	if TradeActive && !dynamicCUActive && time.Since(LastTradeTime) >= time.Minute {
		// If a trade has been active and tracking is off and trading has been at least 1 minute
		dynamicCUActive = true
	} else if !TradeActive && dynamicCUActive {
		// If no trade activity and tracking is active for the inactivity timeout period, stop tracking
		dynamicCUActive = false
	}
}

// Adjust the CU price multiplier based on the landing rate
func updateCUPriceMultiplier(landingRate float64, config configLoad.Config) {
	if landingRate < config.TargetMinLandingRate {
		// Increase CU price multiplier if landing rate is too low
		CurrentMultiplier *= config.PriceAdjustmentFactor
		log.Printf("Landing rate %.2f is below %.2f, increasing multiplier to %.2f", landingRate, config.TargetMinLandingRate, CurrentMultiplier)
	} else if landingRate > config.TargetMaxLandingRate {
		// Decrease CU price multiplier if landing rate is too high
		CurrentMultiplier /= config.PriceAdjustmentFactor
		log.Printf("Landing rate %.2f is above %.2f, decreasing multiplier to %.2f", landingRate, config.TargetMaxLandingRate, CurrentMultiplier)
	} else {
		// Landing rate is within the target range, so no change
		log.Printf("Landing rate %.2f is within the target range, no change", landingRate)
	}
}

func StartDynamicCULoop(config configLoad.Config) {
	go func() {
		ticker := time.NewTicker(time.Duration(config.DynamicLoopInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				UpdateTrackingState()
				if dynamicCUActive {
					landingRate := float64(len(LandedSlots)) / float64(config.SlotsToCheck)
					updateCUPriceMultiplier(landingRate, config)
				}
			}
		}
	}()
}
