package main

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/ethvanity/generator"
)

const (
	version    = "0.1" // Updated for GPU support
	outputFile = "wallet.txt"
	updateRate = 33 * time.Millisecond // Faster updates for smoother animation
)

func main() {
	// Check for test mode
	if len(os.Args) > 1 && os.Args[1] == "--test" {
		runGPUTests()
		return
	}

	// Clear screen and show banner
	clearScreen()
	printWelcomeBanner()

	// Select generator (CPU/GPU)
	gen := selectGenerator()

	// Main application loop - allows searching for multiple addresses
	for {
		// Interactive prompts for prefix/suffix
		prefix, suffix := getInputFromUser()

		// Validate at least one is provided
		if prefix == "" && suffix == "" {
			fmt.Printf("\n    %s✗ Must specify prefix or suffix!%s\n", colorRed, colorReset)
			continue
		}

		// Create configuration
		config := &generator.Config{
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
		printSearchInfo(config, gen)
		difficulty := estimateDifficulty(prefix, suffix)

		// Start the generator
		resultChan, err := gen.Start(ctx, config)
		if err != nil {
			fmt.Printf("\n    %s✗ Error: %v%s\n", colorRed, err, colorReset)
			cancel()
			signal.Stop(sigChan)
			waitForExit()
			return
		}

		startTime := time.Now()

		// Start progress display ticker
		ticker := time.NewTicker(updateRate)

		// Animation frame counter
		frame := 0

		// Search loop for current pattern
		searchDone := false

		for !searchDone {
			select {
			case result := <-resultChan:
				// Found a match!
				ticker.Stop()
				elapsed := time.Since(startTime)
				stats := gen.Stats()
				clearLine()
				printSuccess(result, elapsed, stats.Attempts)
				saveResult(result, elapsed, stats.Attempts)
				cancel()
				signal.Stop(sigChan)

				// Ask user if they want to continue
				if !askToContinue() {
					return
				}

				// User wants to continue - break to outer loop for new prefix/suffix
				searchDone = true
				fmt.Println()

			case <-ticker.C:
				// Update progress display with animation
				stats := gen.Stats()
				printProgress(stats, difficulty, frame)
				frame++

			case <-sigChan:
				// User interrupted
				ticker.Stop()
				clearLine()
				elapsed := time.Since(startTime)
				stats := gen.Stats()
				fmt.Println("\n")
				fmt.Printf("    %s⚠ Cancelled%s │ %s attempts │ %s\n",
					colorYellow+colorBold, colorReset,
					formatNumber(stats.Attempts),
					formatDuration(elapsed))
				cancel()
				signal.Stop(sigChan)

				// Ask if they want to try a different pattern
				if !askToContinue() {
					return
				}
				searchDone = true
				fmt.Println()
			}
		}
	}
}

// clearScreen clears the terminal
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorPurple = "\033[35m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// printWelcomeBanner shows the welcome screen
func printWelcomeBanner() {
	fmt.Println()
	fmt.Printf("%s%s", colorCyan, colorBold)
	fmt.Println("  ╔════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║  ██╗  ██╗███████╗██╗  ██╗██╗  ██╗██╗   ██╗███╗  ██╗████████╗███████╗██████╗    ║")
	fmt.Println("  ║  ██║  ██║██╔════╝╚██╗██╔╝██║  ██║██║   ██║████╗ ██║╚══██╔══╝██╔════╝██╔══██╗   ║")
	fmt.Println("  ║  ███████║█████╗   ╚███╔╝ ███████║██║   ██║██╔██╗██║   ██║   █████╗  ██████╔╝   ║")
	fmt.Println("  ║  ██╔══██║██╔══╝   ██╔██╗ ██╔══██║██║   ██║██║╚████║   ██║   ██╔══╝  ██╔══██╗   ║")
	fmt.Println("  ║  ██║  ██║███████╗██╔╝ ██╗██║  ██║╚██████╔╝██║ ╚███║   ██║   ███████╗██║  ██║   ║")
	fmt.Println("  ║  ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚══╝   ╚═╝   ╚══════╝╚═╝  ╚═╝   ║")
	fmt.Println("  ╠════════════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("  ║%s         Ethereum Vanity Address Generator %s• v%s%s                               ║\n", colorYellow, colorDim, version, colorCyan+colorBold)
	fmt.Println("  ╚════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Print(colorReset)
	fmt.Println()
}

// selectGenerator allows user to choose between CPU and GPU
func selectGenerator() generator.Generator {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("    %s⚡ SELECT ENGINE%s\n", colorPurple+colorBold, colorReset)

	// Check GPU availability
	gpuAvailable := false
	gpuInfo := ""
	if gpus, err := generator.GetGPUInfo(); err == nil && len(gpus) > 0 {
		gpuAvailable = true
		gpuInfo = gpus[0].Name
	}

	fmt.Printf("    %s[1]%s 💻 CPU (%d cores)\n", colorCyan, colorReset, runtime.NumCPU())
	if gpuAvailable {
		fmt.Printf("    %s[2]%s 🎮 GPU %s(%s)%s ⚡\n", colorCyan, colorReset, colorDim, gpuInfo, colorReset)
		fmt.Printf("    %s[3]%s 🔬 Test GPU\n", colorCyan, colorReset)
	} else {
		fmt.Printf("    %s[2]%s 🎮 GPU %s(N/A)%s\n", colorCyan, colorReset, colorDim, colorReset)
	}

	fmt.Printf("\n    %s→%s ", colorGreen, colorReset)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	if choice == "3" && gpuAvailable {
		runGPUTests()
		os.Exit(0)
	}

	if choice == "2" && gpuAvailable {
		gpuGen, err := generator.NewGPUGenerator()
		if err != nil {
			fmt.Printf("    %s⚠ GPU failed: %v%s\n", colorRed, err, colorReset)
			fmt.Printf("    %s↪ Using CPU...%s\n", colorYellow, colorReset)
			return generator.NewCPUGenerator(0)
		}
		fmt.Printf("    %s✓ GPU Ready%s\n\n", colorGreen, colorReset)
		return gpuGen
	}

	fmt.Printf("    %s✓ CPU Ready%s\n\n", colorGreen, colorReset)
	return generator.NewCPUGenerator(0)
}

// getInputFromUser prompts user for prefix and suffix
func getInputFromUser() (string, string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("    %s🎯 TARGET PATTERN%s\n", colorPurple+colorBold, colorReset)

	// Get prefix
	fmt.Printf("    %sPrefix%s (0x...): ", colorCyan, colorReset)
	prefixInput, _ := reader.ReadString('\n')
	prefix := strings.TrimSpace(prefixInput)
	prefix = strings.TrimPrefix(strings.ToLower(prefix), "0x")

	// Validate prefix
	if prefix != "" && !isValidHex(prefix) {
		fmt.Printf("    %s⚠ Invalid! Hex only (0-9, a-f)%s\n", colorRed, colorReset)
		prefix = ""
	}

	// Get suffix
	fmt.Printf("    %sSuffix%s (...xxx): ", colorCyan, colorReset)
	suffixInput, _ := reader.ReadString('\n')
	suffix := strings.TrimSpace(suffixInput)
	suffix = strings.ToLower(suffix)

	// Validate suffix
	if suffix != "" && !isValidHex(suffix) {
		fmt.Printf("    %s⚠ Invalid! Hex only (0-9, a-f)%s\n", colorRed, colorReset)
		suffix = ""
	}

	return prefix, suffix
}

// printSearchInfo displays search configuration
func printSearchInfo(config *generator.Config, gen generator.Generator) {
	fmt.Printf("\n    %s🚀 SEARCHING%s", colorGreen+colorBold, colorReset)

	if config.Prefix != "" {
		fmt.Printf(" %s%s0x%s%s...%s", colorBold, colorCyan, config.Prefix, colorDim, colorReset)
	}
	if config.Suffix != "" {
		fmt.Printf("%s...%s%s%s%s", colorDim, colorCyan, colorBold, config.Suffix, colorReset)
	}

	difficulty := estimateDifficulty(config.Prefix, config.Suffix)
	fmt.Printf(" %s(1/%s)%s\n\n", colorDim, formatNumber(difficulty), colorReset)
}

// printProgress shows animated progress bar
func printProgress(stats generator.Stats, difficulty uint64, frame int) {
	// Spinner animation with colors
	spinners := []string{"◐", "◓", "◑", "◒"}
	spinner := spinners[frame%len(spinners)]

	// Progress bar (probability based)
	// Formula: 1 - 0.5^(2 * attempts / difficulty)
	// At attempts = difficulty/2, progress = 1 - 0.5^1 = 0.5 (50%)
	// Asymptotically approaches 1.0, slowing down as it fills

	attempts := float64(stats.Attempts)
	diff := float64(difficulty)
	if diff == 0 {
		diff = 1
	}

	ratio := attempts / diff
	// We want 50% bar at 50% probability (attempts = difficulty/2, so ratio = 0.5)
	// 0.5 = 1 - 0.5^(k * 0.5) => 0.5 = 0.5^(0.5k) => 1 = 0.5k => k = 2
	progress := 1.0 - math.Pow(0.5, 2.0*ratio)

	barWidth := 40
	filled := int(progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("▓", filled) + strings.Repeat("░", barWidth-filled)

	// Format speed
	speedStr := formatHashRate(stats.HashRate)

	// Format output with colors
	fmt.Printf("\r    %s%s%s %s%s%s %s%s%s │ %s%s%s │ %s",
		colorCyan, spinner, colorReset,
		colorDim, bar, colorReset,
		colorGreen+colorBold, speedStr, colorReset,
		colorYellow, formatNumber(stats.Attempts), colorReset,
		formatDuration(time.Duration(stats.ElapsedSecs*float64(time.Second))))
}

// formatHashRate formats hash rate nicely
func formatHashRate(rate float64) string {
	if rate >= 1000000 {
		return fmt.Sprintf("%.1fM/s", rate/1000000)
	}
	if rate >= 1000 {
		return fmt.Sprintf("%.1fK/s", rate/1000)
	}
	return fmt.Sprintf("%.0f/s", rate)
}

// printSuccess shows the found address
func printSuccess(result generator.Result, elapsed time.Duration, attempts uint64) {
	fmt.Printf("\n    %s%s╔══════════════════════════════════════════════════════════╗%s\n", colorGreen, colorBold, colorReset)
	fmt.Printf("    %s%s║               ✨ ADDRESS FOUND! ✨                       ║%s\n", colorGreen, colorBold, colorReset)
	fmt.Printf("    %s%s╚══════════════════════════════════════════════════════════╝%s\n\n", colorGreen, colorBold, colorReset)

	fmt.Printf("    %s📍 ADDRESS%s\n", colorCyan+colorBold, colorReset)
	fmt.Println()
	fmt.Printf("       %s%s%s%s\n", colorGreen, colorBold, result.Address, colorReset)
	fmt.Println()

	fmt.Printf("    %s🔑 PRIVATE KEY%s\n", colorPurple+colorBold, colorReset)
	fmt.Printf("       %s%s%s\n\n", colorYellow, result.PrivateKey, colorReset)

	fmt.Printf("    %s⏱   %s%s   %s│   %s📊  %s%s   %s│   %s💾  %s%s%s\n\n",
		colorCyan, colorReset+colorBold, formatDuration(elapsed),
		colorDim,
		colorPurple, colorReset+colorBold, formatNumber(attempts),
		colorDim,
		colorYellow, colorReset+colorBold, outputFile,
		colorReset)
	fmt.Printf("    %s%s⚠  KEEP YOUR PRIVATE KEY SECRET!%s\n", colorRed, colorBold, colorReset)
}

// saveResult writes the result to a file
func saveResult(result generator.Result, elapsed time.Duration, attempts uint64) {
	content := fmt.Sprintf(`Ethereum Vanity Address
=======================

Address:     %s
Private Key: %s

Statistics:
  Time:     %s
  Attempts: %s

Generated: %s

⚠️ WARNING: Keep this private key secret and secure!
`, result.Address, result.PrivateKey, formatDuration(elapsed), formatNumber(attempts), time.Now().Format(time.RFC3339))

	err := os.WriteFile(outputFile, []byte(content), 0600)
	if err != nil {
		fmt.Printf("    %s⚠ Save failed: %v%s\n", colorYellow, err, colorReset)
	}
}

// clearLine clears the current line
func clearLine() {
	fmt.Print("\r                                                                                              \r")
}

// waitForExit waits for user to press Enter before exiting
func waitForExit() {
	fmt.Printf("\n    %sPress Enter to exit...%s", colorDim, colorReset)
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

// askToContinue prompts user to continue or exit
func askToContinue() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n    %s[Enter]%s Continue searching  │  %s[Q]%s Exit\n", colorGreen, colorReset, colorRed, colorReset)
	fmt.Printf("    %s→%s ", colorCyan, colorReset)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input != "q" && input != "quit" && input != "exit"
}

// isValidHex checks if string contains only hex characters
func isValidHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// estimateDifficulty calculates expected attempts
func estimateDifficulty(prefix, suffix string) uint64 {
	totalChars := len(prefix) + len(suffix)
	if totalChars == 0 {
		return 1
	}
	difficulty := uint64(1)
	for i := 0; i < totalChars; i++ {
		difficulty *= 16
	}
	return difficulty
}

// formatNumber adds commas to large numbers
func formatNumber(n uint64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// formatDuration formats duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

// runGPUTests runs GPU verification tests
func runGPUTests() {
	fmt.Println()
	fmt.Println("  ╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║              🔬 GPU Verification Test (Phase 1)                   ║")
	fmt.Println("  ╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("  🔄 Running GPU vs CPU comparison tests...")
	fmt.Println()

	passed, results := generator.VerifyGPUImplementation()

	// Print results
	for i, r := range results {
		fmt.Printf("  Test %d: %s\n", i+1, r.TestName)
		if r.ErrorMessage != "" {
			fmt.Printf("    ❌ Error: %s\n", r.ErrorMessage)
		} else {
			keyPreview := r.PrivateKey
			if len(keyPreview) > 16 {
				keyPreview = keyPreview[:8] + "..." + keyPreview[len(keyPreview)-8:]
			}
			fmt.Printf("    🔑 Private Key: %s\n", keyPreview)
			fmt.Printf("    💻 CPU Address: %s\n", r.CPUAddress)
			fmt.Printf("    🎮 GPU Address: %s\n", r.GPUAddress)
			if r.Match {
				fmt.Printf("    ✅ MATCH!\n")
			} else {
				fmt.Printf("    ❌ MISMATCH!\n")
			}
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	if passed {
		fmt.Println("  ✅ ALL TESTS PASSED! GPU implementation is correct.")
		fmt.Println("  ➡️  Ready for Phase 2: Parallelization")
	} else {
		fmt.Println("  ❌ SOME TESTS FAILED! GPU implementation needs fixing.")
		fmt.Println("  Review the mismatches above to debug.")
	}
	fmt.Println()

	if !passed {
		waitForExit()
		os.Exit(1)
	}
	waitForExit()
}
