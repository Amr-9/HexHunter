package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Amr-9/HexHunter/pkg/generator"
)

// ANSI color codes
const (
	ColorReset  = "\033[0m"
	ColorCyan   = "\033[36m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorRed    = "\033[31m"
	ColorPurple = "\033[35m"
	ColorBold   = "\033[1m"
	ColorDim    = "\033[2m"
)

// ClearScreen clears the terminal
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

// PrintWelcomeBanner shows the welcome screen
func PrintWelcomeBanner(version string) {
	fmt.Println()
	fmt.Printf("%s%s", ColorCyan, ColorBold)
	fmt.Println("  ╔════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║  ██╗  ██╗███████╗██╗  ██╗██╗  ██╗██╗   ██╗███╗  ██╗████████╗███████╗██████╗    ║")
	fmt.Println("  ║  ██║  ██║██╔════╝╚██╗██╔╝██║  ██║██║   ██║████╗ ██║╚══██╔══╝██╔════╝██╔══██╗   ║")
	fmt.Println("  ║  ███████║█████╗   ╚███╔╝ ███████║██║   ██║██╔██╗██║   ██║   █████╗  ██████╔╝   ║")
	fmt.Println("  ║  ██╔══██║██╔══╝   ██╔██╗ ██╔══██║██║   ██║██║╚████║   ██║   ██╔══╝  ██╔══██╗   ║")
	fmt.Println("  ║  ██║  ██║███████╗██╔╝ ██╗██║  ██║╚██████╔╝██║ ╚███║   ██║   ███████╗██║  ██║   ║")
	fmt.Println("  ║  ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚══╝   ╚═╝   ╚══════╝╚═╝  ╚═╝   ║")
	fmt.Println("  ╠════════════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("  ║%s         Vanity Address Generator %s• v%s%s                                        ║\n", ColorYellow, ColorDim, version, ColorCyan+ColorBold)
	fmt.Println("  ╚════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Print(ColorReset)
	fmt.Println()
}

// PrintSearchInfo displays search configuration
func PrintSearchInfo(config *generator.Config, difficulty uint64) {
	fmt.Printf("\n    %s🚀 SEARCHING%s", ColorGreen+ColorBold, ColorReset)

	switch config.Network {
	case generator.Solana:
		// Solana format (no prefix)
		if config.Prefix != "" {
			fmt.Printf(" %s%s%s%s...%s", ColorBold, ColorCyan, config.Prefix, ColorDim, ColorReset)
		}
		if config.Suffix != "" {
			fmt.Printf("%s...%s%s%s%s", ColorDim, ColorCyan, ColorBold, config.Suffix, ColorReset)
		}
	case generator.Bitcoin:
		// Bitcoin format (bc1p, 1, or 3 prefix based on address type)
		prefix := "bc1p"
		switch SelectedBitcoinAddressType {
		case generator.AddressTypeLegacy:
			prefix = "1"
		case generator.AddressTypeNestedSegWit:
			prefix = "3"
		}
		if config.Prefix != "" {
			fmt.Printf(" %s%s%s%s%s...%s", ColorBold, ColorCyan, prefix, config.Prefix, ColorDim, ColorReset)
		} else {
			fmt.Printf(" %s%s%s%s...%s", ColorBold, ColorCyan, prefix, ColorDim, ColorReset)
		}
		if config.Suffix != "" {
			fmt.Printf("%s...%s%s%s%s", ColorDim, ColorCyan, ColorBold, config.Suffix, ColorReset)
		}
	default:
		// Ethereum/Aptos/Sui format (0x prefix)
		if config.Prefix != "" {
			fmt.Printf(" %s%s0x%s%s...%s", ColorBold, ColorCyan, config.Prefix, ColorDim, ColorReset)
		}
		if config.Suffix != "" {
			fmt.Printf("%s...%s%s%s%s", ColorDim, ColorCyan, ColorBold, config.Suffix, ColorReset)
		}
	}

	fmt.Printf(" %s(1/%s)%s\n\n", ColorDim, FormatNumber(difficulty), ColorReset)
}

// PrintProgress shows animated progress bar
func PrintProgress(stats generator.Stats, difficulty uint64, frame int) {
	spinners := []string{"◐", "◓", "◑", "◒"}
	spinner := spinners[frame%len(spinners)]

	attempts := float64(stats.Attempts)
	diff := float64(difficulty)
	if diff == 0 {
		diff = 1
	}

	ratio := attempts / diff
	progress := 1.0 - math.Pow(0.5, 2.0*ratio)

	barWidth := 40
	filled := int(progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("▓", filled) + strings.Repeat("░", barWidth-filled)

	speedStr := FormatHashRate(stats.HashRate)

	fmt.Printf("\r    %s%s%s %s%s%s %s%s%s │ %s%s%s │ %s",
		ColorCyan, spinner, ColorReset,
		ColorDim, bar, ColorReset,
		ColorGreen+ColorBold, speedStr, ColorReset,
		ColorYellow, FormatNumber(stats.Attempts), ColorReset,
		FormatDuration(time.Duration(stats.ElapsedSecs*float64(time.Second))))
}

// FormatHashRate formats hash rate nicely
func FormatHashRate(rate float64) string {
	if rate >= 1000000 {
		return fmt.Sprintf("%.1fM/s", rate/1000000)
	}
	if rate >= 1000 {
		return fmt.Sprintf("%.1fK/s", rate/1000)
	}
	return fmt.Sprintf("%.0f/s", rate)
}

// PrintSuccess shows the found address
func PrintSuccess(result generator.Result, elapsed time.Duration, attempts uint64, outputFile string) {
	fmt.Printf("\n    %s%s╔══════════════════════════════════════════════════════════╗%s\n", ColorGreen, ColorBold, ColorReset)
	fmt.Printf("    %s%s║               ✨ ADDRESS FOUND! ✨                       ║%s\n", ColorGreen, ColorBold, ColorReset)
	fmt.Printf("    %s%s╚══════════════════════════════════════════════════════════╝%s\n\n", ColorGreen, ColorBold, ColorReset)

	networkLabel := "📍 ADDRESS"
	switch result.Network {
	case generator.Solana:
		networkLabel = "◎ SOLANA ADDRESS"
	case generator.Aptos:
		networkLabel = "◆ APTOS ADDRESS"
	case generator.Sui:
		networkLabel = "◇ SUI ADDRESS"
	case generator.Bitcoin:
		networkLabel = "₿ BITCOIN ADDRESS"
	default:
		networkLabel = "⟠ ETHEREUM ADDRESS"
	}

	fmt.Printf("    %s%s%s\n", ColorCyan+ColorBold, networkLabel, ColorReset)
	fmt.Println()
	fmt.Printf("       %s%s%s%s\n", ColorGreen, ColorBold, result.Address, ColorReset)
	fmt.Println()

	fmt.Printf("    %s🔑 PRIVATE KEY%s\n", ColorPurple+ColorBold, ColorReset)
	fmt.Printf("       %s%s%s\n\n", ColorYellow, result.PrivateKey, ColorReset)

	fmt.Printf("    %s⏱   %s%s   %s│   %s📊  %s%s   %s│   %s💾  %s%s%s\n\n",
		ColorCyan, ColorReset+ColorBold, FormatDuration(elapsed),
		ColorDim,
		ColorPurple, ColorReset+ColorBold, FormatNumber(attempts),
		ColorDim,
		ColorYellow, ColorReset+ColorBold, outputFile,
		ColorReset)
	fmt.Printf("    %s%s⚠  KEEP YOUR PRIVATE KEY SECRET!%s\n", ColorRed, ColorBold, ColorReset)
}

// ClearLine clears the current line
func ClearLine() {
	fmt.Print("\r                                                                                              \r")
}

// WaitForExit waits for user to press Enter before exiting
func WaitForExit() {
	fmt.Printf("\n    %sPress Enter to exit...%s", ColorDim, ColorReset)
	// Use fmt.Scan to clear any pending input if needed, but primarily wait for user
	var input string
	fmt.Scanln(&input)
}

// FormatNumber adds commas to large numbers
func FormatNumber(n uint64) string {
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

// FormatDuration formats duration in a human-readable way
func FormatDuration(d time.Duration) string {
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
