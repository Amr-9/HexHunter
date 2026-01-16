package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Amr-9/HexHunter/internal/ui"
	"github.com/Amr-9/HexHunter/pkg/generator"
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

	// Select engine (CPU/GPU) and network initially
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
			Network:     currentNetwork,
			AddressType: ui.SelectedBitcoinAddressType, // Used by Bitcoin
			Prefix:      prefix,
			Suffix:      suffix,
			Workers:     runtime.NumCPU(),
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

				action := ui.AskToContinue()
				switch action {
				case ui.ActionQuit:
					return
				case ui.ActionSwitchNetwork:
					fmt.Println()
					gen, network = ui.SelectNetworkOnly()
					currentNetwork = network
				}
				searchDone = true

			case <-ticker.C:
				stats := gen.Stats()
				ui.PrintProgress(stats, difficulty, frame)
				frame++

			case <-sigChan:
				ticker.Stop()
				ui.ClearLine()
				elapsed := time.Since(startTime)
				stats := gen.Stats()
				fmt.Println()
				fmt.Printf("    %s⚠ Cancelled%s │ %s attempts │ %s\n",
					ui.ColorYellow+ui.ColorBold, ui.ColorReset,
					ui.FormatNumber(stats.Attempts),
					ui.FormatDuration(elapsed))
				cancel()
				signal.Stop(sigChan)

				action := ui.AskToContinue()
				switch action {
				case ui.ActionQuit:
					return
				case ui.ActionSwitchNetwork:
					fmt.Println()
					gen, network = ui.SelectNetworkOnly()
					currentNetwork = network
				}
				searchDone = true
			}
		}
	}
}

// saveResult writes the result to a file
func saveResult(result generator.Result, elapsed time.Duration, attempts uint64) {
	networkName := result.Network.String()

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

	switch network {
	case generator.Solana:
		base = 58 // Base58 for Solana
	case generator.Tron:
		base = 58 // Base58 for Tron
	case generator.Bitcoin:
		// Bitcoin address type determines encoding:
		// - Taproot (P2TR) / Native SegWit: Bech32/Bech32m = 32 chars
		// - Legacy (P2PKH) / Nested SegWit (P2SH): Base58 = 58 chars
		switch ui.SelectedBitcoinAddressType {
		case generator.AddressTypeTaproot:
			base = 32 // Bech32m
		case generator.AddressTypeLegacy, generator.AddressTypeNestedSegWit:
			base = 58 // Base58
		default:
			base = 32 // Default to Bech32
		}
	}

	for i := 0; i < totalChars; i++ {
		difficulty *= base
	}
	return difficulty
}
