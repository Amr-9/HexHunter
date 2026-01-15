package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/ethvanity/internal/ui"
	"github.com/ethvanity/pkg/generator"
)

const (
	version    = "0.3" // Updated for Solana support
	outputFile = "wallet.txt"
	updateRate = 33 * time.Millisecond
)

// Global state for current network
var currentNetwork generator.Network = generator.Ethereum

func main() {
	// Clear screen and show banner
	ui.ClearScreen()
	ui.PrintWelcomeBanner(version)

	// Select engine (CPU/GPU) and network (Ethereum/Solana)
	gen, network := ui.SelectEngineAndNetwork()
	if gen == nil {
		os.Exit(0)
	}
	currentNetwork = network

	// Main application loop
	for {
		// Interactive prompts for prefix/suffix
		prefix, suffix := ui.GetInputFromUser(currentNetwork)

		// Validate at least one is provided
		if prefix == "" && suffix == "" {
			fmt.Printf("\n    %s✗ Must specify prefix or suffix!%s\n", ui.ColorRed, ui.ColorReset)
			continue
		}

		// Create configuration
		config := &generator.Config{
			Network: currentNetwork,
			Prefix:  prefix,
			Suffix:  suffix,
			Workers: runtime.NumCPU(),
		}

		// Setup context with cancellation
		ctx, cancel := context.WithCancel(context.Background())

		// Handle interrupt signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Print search info
		difficulty := estimateDifficulty(config.Prefix, config.Suffix, currentNetwork)
		ui.PrintSearchInfo(config, difficulty)

		// Start the generator
		resultChan, err := gen.Start(ctx, config)
		if err != nil {
			fmt.Printf("\n    %s✗ Error: %v%s\n", ui.ColorRed, err, ui.ColorReset)
			cancel()
			signal.Stop(sigChan)
			ui.WaitForExit()
			return
		}

		startTime := time.Now()
		ticker := time.NewTicker(updateRate)
		frame := 0
		searchDone := false

		for !searchDone {
			select {
			case result := <-resultChan:
				ticker.Stop()
				elapsed := time.Since(startTime)
				stats := gen.Stats()
				ui.ClearLine()
				ui.PrintSuccess(result, elapsed, stats.Attempts, outputFile)
				saveResult(result, elapsed, stats.Attempts)
				cancel()
				signal.Stop(sigChan)

				if !ui.AskToContinue() {
					return
				}
				searchDone = true
				fmt.Println()

			case <-ticker.C:
				stats := gen.Stats()
				ui.PrintProgress(stats, difficulty, frame)
				frame++

			case <-sigChan:
				ticker.Stop()
				ui.ClearLine()
				elapsed := time.Since(startTime)
				stats := gen.Stats()
				fmt.Println("\n")
				fmt.Printf("    %s⚠ Cancelled%s │ %s attempts │ %s\n",
					ui.ColorYellow+ui.ColorBold, ui.ColorReset,
					ui.FormatNumber(stats.Attempts),
					ui.FormatDuration(elapsed))
				cancel()
				signal.Stop(sigChan)

				if !ui.AskToContinue() {
					return
				}
				searchDone = true
				fmt.Println()
			}
		}
	}
}

// saveResult writes the result to a file
func saveResult(result generator.Result, elapsed time.Duration, attempts uint64) {
	networkName := "Ethereum"
	if result.Network == generator.Solana {
		networkName = "Solana"
	}

	content := fmt.Sprintf(`%s Vanity Address
=======================

Address:     %s
Private Key: %s

Statistics:
  Time:     %s
  Attempts: %s

Generated: %s

⚠️ WARNING: Keep this private key secret and secure!
`, networkName, result.Address, result.PrivateKey, ui.FormatDuration(elapsed), ui.FormatNumber(attempts), time.Now().Format("2006-01-02 15:04:05"))

	err := os.WriteFile(outputFile, []byte(content), 0600)
	if err != nil {
		fmt.Printf("    %s⚠ Save failed: %v%s\n", ui.ColorYellow, err, ui.ColorReset)
	}
}

// estimateDifficulty calculates expected attempts based on network
func estimateDifficulty(prefix, suffix string, network generator.Network) uint64 {
	totalChars := len(prefix) + len(suffix)
	if totalChars == 0 {
		return 1
	}

	difficulty := uint64(1)
	base := uint64(16) // Hex for Ethereum

	if network == generator.Solana {
		base = 58 // Base58 for Solana
	}

	for i := 0; i < totalChars; i++ {
		difficulty *= base
	}
	return difficulty
}
