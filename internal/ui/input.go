package ui

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/Amr-9/HexHunter/pkg/generator"
	"github.com/Amr-9/HexHunter/pkg/generator/aptos"
	"github.com/Amr-9/HexHunter/pkg/generator/bitcoin"
	"github.com/Amr-9/HexHunter/pkg/generator/cpu"
	"github.com/Amr-9/HexHunter/pkg/generator/ethereum"
	"github.com/Amr-9/HexHunter/pkg/generator/solana"
	"github.com/Amr-9/HexHunter/pkg/generator/tron"
)

// SelectedBitcoinAddressType holds the selected Bitcoin address type (global for simplicity)
var SelectedBitcoinAddressType generator.AddressType = generator.AddressTypeTaproot

// selectedUseGPU tracks whether GPU was selected (for network switching)
var selectedUseGPU bool = false

// SelectEngineAndNetwork handles the engine (CPU/GPU) and network (Ethereum/Solana/Bitcoin) selection.
// Returns the appropriate generator and selected network.
func SelectEngineAndNetwork() (generator.Generator, generator.Network) {
	reader := bufio.NewReader(os.Stdin)

	// Step 1: Select Engine
	fmt.Printf("    %s⚡ SELECT ENGINE%s\n", ColorPurple+ColorBold, ColorReset)

	gpuAvailable := false
	gpuInfo := ""
	// Using ethereum stub for now as it contains the stubs/impl for GPU checks generally
	if gpus, err := ethereum.GetGPUInfo(); err == nil && len(gpus) > 0 {
		gpuAvailable = true
		gpuInfo = gpus[0].Name
	}

	fmt.Printf("    %s[1]%s 💻 CPU (%d cores)\n", ColorCyan, ColorReset, runtime.NumCPU())
	if gpuAvailable {
		fmt.Printf("    %s[2]%s 🎮 GPU %s(%s)%s ⚡\n", ColorCyan, ColorReset, ColorDim, gpuInfo, ColorReset)
	} else {
		fmt.Printf("    %s[2]%s 🎮 GPU %s(N/A)%s\n", ColorCyan, ColorReset, ColorDim, ColorReset)
	}

	fmt.Printf("\n    %s→%s ", ColorGreen, ColorReset)
	engineChoice, _ := reader.ReadString('\n')
	engineChoice = strings.TrimSpace(engineChoice)

	selectedUseGPU = engineChoice == "2" && gpuAvailable
	if selectedUseGPU {
		fmt.Printf("    %s✓ GPU Ready%s\n\n", ColorGreen, ColorReset)
	} else {
		fmt.Printf("    %s✓ CPU Ready%s\n\n", ColorGreen, ColorReset)
	}

	return selectNetworkWithEngine(reader, selectedUseGPU)
}

// SelectNetworkOnly allows switching network without re-selecting engine.
// Uses the previously selected engine (CPU/GPU).
func SelectNetworkOnly() (generator.Generator, generator.Network) {
	reader := bufio.NewReader(os.Stdin)
	return selectNetworkWithEngine(reader, selectedUseGPU)
}

// selectNetworkWithEngine handles the network selection with a specified engine.
func selectNetworkWithEngine(reader *bufio.Reader, useGPU bool) (generator.Generator, generator.Network) {
	fmt.Printf("    %s🌐 SELECT NETWORK%s\n", ColorPurple+ColorBold, ColorReset)
	fmt.Printf("    %s[1]%s ⟠ Ethereum (ETH) %s- 0x prefix, Hex%s", ColorCyan, ColorReset, ColorDim, ColorReset)
	if useGPU {
		fmt.Printf(" ⚡\n")
	} else {
		fmt.Printf("\n")
	}
	fmt.Printf("    %s[2]%s ◎ Solana (SOL) %s- Base58%s", ColorCyan, ColorReset, ColorDim, ColorReset)
	if useGPU {
		fmt.Printf(" ⚡\n")
	} else {
		fmt.Printf("\n")
	}
	fmt.Printf("    %s[3]%s ◆ Aptos (APT) %s- 0x prefix, Hex%s", ColorCyan, ColorReset, ColorDim, ColorReset)
	if useGPU {
		fmt.Printf(" ⚡\n")
	} else {
		fmt.Printf("\n")
	}
	fmt.Printf("    %s[4]%s ◇ Sui (SUI) %s- 0x prefix, Hex%s", ColorCyan, ColorReset, ColorDim, ColorReset)
	fmt.Printf(" %s(CPU only)%s\n", ColorDim, ColorReset)
	fmt.Printf("    %s[5]%s ₿ Bitcoin (BTC) %s- Taproot/Legacy/SegWit%s", ColorCyan, ColorReset, ColorDim, ColorReset)
	fmt.Printf(" %s(CPU only)%s\n", ColorDim, ColorReset)
	fmt.Printf("    %s[6]%s ₮ Tron (TRX) %s- Base58, T prefix%s", ColorCyan, ColorReset, ColorDim, ColorReset)
	if useGPU {
		fmt.Printf(" ⚡\n")
	} else {
		fmt.Printf("\n")
	}

	fmt.Printf("\n    %s→%s ", ColorGreen, ColorReset)
	networkChoice, _ := reader.ReadString('\n')
	networkChoice = strings.TrimSpace(networkChoice)

	// Create the appropriate generator
	var gen generator.Generator
	var network generator.Network

	switch networkChoice {
	case "2": // Solana
		network = generator.Solana
		fmt.Printf("    %s✓ Solana Selected%s\n\n", ColorGreen, ColorReset)
		if useGPU {
			solGPU, err := solana.NewSolanaGPUGenerator()
			if err != nil {
				fmt.Printf("    %s⚠ Solana GPU failed: %v%s\n", ColorRed, err, ColorReset)
				fmt.Printf("    %s↪ Using CPU...%s\n", ColorYellow, ColorReset)
				gen = cpu.NewCPUGenerator(0)
			} else {
				gen = solGPU
			}
		} else {
			gen = cpu.NewCPUGenerator(0)
		}
	case "3": // Aptos
		network = generator.Aptos
		fmt.Printf("    %s✓ Aptos Selected%s\n\n", ColorGreen, ColorReset)
		if useGPU {
			aptosGPU, err := aptos.NewAptosGPUGenerator()
			if err != nil {
				fmt.Printf("    %s⚠ Aptos GPU failed: %v%s\n", ColorRed, err, ColorReset)
				fmt.Printf("    %s↪ Using CPU...%s\n", ColorYellow, ColorReset)
				gen = cpu.NewCPUGenerator(0)
			} else {
				gen = aptosGPU
			}
		} else {
			gen = cpu.NewCPUGenerator(0)
		}
	case "4": // Sui
		network = generator.Sui
		fmt.Printf("    %s✓ Sui Selected%s\n\n", ColorGreen, ColorReset)
		// Sui is CPU-only for now
		gen = cpu.NewCPUGenerator(0)
	case "5": // Bitcoin
		network = generator.Bitcoin
		fmt.Printf("    %s✓ Bitcoin Selected%s\n\n", ColorGreen, ColorReset)
		// Bitcoin is CPU-only for now
		gen = cpu.NewCPUGenerator(0)
		// Select address type
		SelectedBitcoinAddressType = selectBitcoinAddressType(reader)
	case "6": // Tron
		network = generator.Tron
		fmt.Printf("    %s✓ Tron Selected%s\n\n", ColorGreen, ColorReset)
		if useGPU {
			tronGPU, err := tron.NewTronGPUGenerator()
			if err != nil {
				fmt.Printf("    %s⚠ Tron GPU failed: %v%s\n", ColorRed, err, ColorReset)
				fmt.Printf("    %s↪ Using CPU...%s\n", ColorYellow, ColorReset)
				gen = cpu.NewCPUGenerator(0)
			} else {
				gen = tronGPU
			}
		} else {
			gen = cpu.NewCPUGenerator(0)
		}
	default: // Ethereum
		network = generator.Ethereum
		fmt.Printf("    %s✓ Ethereum Selected%s\n\n", ColorGreen, ColorReset)
		if useGPU {
			ethGPU, err := ethereum.NewGPUGenerator()
			if err != nil {
				fmt.Printf("    %s⚠ GPU failed: %v%s\n", ColorRed, err, ColorReset)
				fmt.Printf("    %s↪ Using CPU...%s\n", ColorYellow, ColorReset)
				gen = cpu.NewCPUGenerator(0)
			} else {
				gen = ethGPU
			}
		} else {
			gen = cpu.NewCPUGenerator(0)
		}
	}

	return gen, network
}

// GetInputFromUser prompts user for prefix and suffix
func GetInputFromUser(network generator.Network) (string, string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("    %s🎯 TARGET PATTERN%s\n", ColorPurple+ColorBold, ColorReset)

	switch network {
	case generator.Solana:
		return getSolanaInput(reader)
	case generator.Aptos, generator.Sui:
		return getAptosInput(reader)
	case generator.Bitcoin:
		return getBitcoinInput(reader, SelectedBitcoinAddressType)
	case generator.Tron:
		return getTronInput(reader)
	default:
		return getEthereumInput(reader)
	}
}

func getEthereumInput(reader *bufio.Reader) (string, string) {
	fmt.Printf("    %sPrefix%s (0x...): ", ColorCyan, ColorReset)
	prefixInput, _ := reader.ReadString('\n')
	prefix := strings.TrimSpace(prefixInput)
	prefix = strings.TrimPrefix(strings.ToLower(prefix), "0x")

	if prefix != "" && !isValidHex(prefix) {
		fmt.Printf("    %s⚠ Invalid! Hex only (0-9, a-f)%s\n", ColorRed, ColorReset)
		prefix = ""
	}

	fmt.Printf("    %sSuffix%s (...xxx): ", ColorCyan, ColorReset)
	suffixInput, _ := reader.ReadString('\n')
	suffix := strings.TrimSpace(suffixInput)
	suffix = strings.ToLower(suffix)

	if suffix != "" && !isValidHex(suffix) {
		fmt.Printf("    %s⚠ Invalid! Hex only (0-9, a-f)%s\n", ColorRed, ColorReset)
		suffix = ""
	}

	return prefix, suffix
}

func getSolanaInput(reader *bufio.Reader) (string, string) {
	fmt.Printf("    %sPrefix%s (...): ", ColorCyan, ColorReset)
	prefixInput, _ := reader.ReadString('\n')
	prefix := strings.TrimSpace(prefixInput)

	if prefix != "" && !solana.IsValidBase58(prefix) {
		invalidChars := solana.InvalidBase58Chars(prefix)
		fmt.Printf("    %s⚠ Invalid Base58 character(s): %s%s\n", ColorRed, string(invalidChars), ColorReset)
		fmt.Printf("    %s  (Not allowed: 0, O, I, l)%s\n", ColorDim, ColorReset)
		prefix = ""
	}

	fmt.Printf("    %sSuffix%s (...): ", ColorCyan, ColorReset)
	suffixInput, _ := reader.ReadString('\n')
	suffix := strings.TrimSpace(suffixInput)

	if suffix != "" && !solana.IsValidBase58(suffix) {
		invalidChars := solana.InvalidBase58Chars(suffix)
		fmt.Printf("    %s⚠ Invalid Base58 character(s): %s%s\n", ColorRed, string(invalidChars), ColorReset)
		fmt.Printf("    %s  (Not allowed: 0, O, I, l)%s\n", ColorDim, ColorReset)
		suffix = ""
	}

	return prefix, suffix
}

func getAptosInput(reader *bufio.Reader) (string, string) {
	fmt.Printf("    %sPrefix%s (0x...): ", ColorCyan, ColorReset)
	prefixInput, _ := reader.ReadString('\n')
	prefix := strings.TrimSpace(prefixInput)
	prefix = strings.TrimPrefix(strings.ToLower(prefix), "0x")

	if prefix != "" && !isValidHex(prefix) {
		fmt.Printf("    %s⚠ Invalid! Hex only (0-9, a-f)%s\n", ColorRed, ColorReset)
		prefix = ""
	}

	fmt.Printf("    %sSuffix%s (...xxx): ", ColorCyan, ColorReset)
	suffixInput, _ := reader.ReadString('\n')
	suffix := strings.TrimSpace(suffixInput)
	suffix = strings.ToLower(suffix)

	if suffix != "" && !isValidHex(suffix) {
		fmt.Printf("    %s⚠ Invalid! Hex only (0-9, a-f)%s\n", ColorRed, ColorReset)
		suffix = ""
	}

	return prefix, suffix
}

// ContinueAction represents what the user wants to do after finding an address
type ContinueAction int

const (
	ActionContinue      ContinueAction = iota // Continue with same pattern
	ActionQuit                                // Exit the application
	ActionSwitchNetwork                       // Switch to different network
)

// AskToContinue prompts user to continue, switch network, or exit
func AskToContinue() ContinueAction {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n    %s[Enter]%s Continue  │  %s[N]%s New Network  │  %s[Q]%s Exit\n",
		ColorGreen, ColorReset, ColorCyan, ColorReset, ColorRed, ColorReset)
	fmt.Printf("    %s→%s ", ColorCyan, ColorReset)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "q", "quit", "exit":
		return ActionQuit
	case "n", "new", "network":
		return ActionSwitchNetwork
	default:
		return ActionContinue
	}
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

// selectBitcoinAddressType prompts user to select a Bitcoin address type.
func selectBitcoinAddressType(reader *bufio.Reader) generator.AddressType {
	fmt.Printf("    %s🔧 SELECT ADDRESS TYPE%s\n", ColorPurple+ColorBold, ColorReset)
	fmt.Printf("    %s[1]%s ⚡ Taproot (bc1p...) %s- Recommended, Bech32m%s\n", ColorCyan, ColorReset, ColorGreen, ColorReset)
	fmt.Printf("    %s[2]%s 🏛️  Legacy (1...) %s- Base58, Case-sensitive%s\n", ColorCyan, ColorReset, ColorDim, ColorReset)
	fmt.Printf("    %s[3]%s 📦 Nested SegWit (3...) %s- Base58, Case-sensitive%s\n", ColorCyan, ColorReset, ColorDim, ColorReset)

	fmt.Printf("\n    %s→%s ", ColorGreen, ColorReset)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "2":
		fmt.Printf("    %s✓ Legacy (P2PKH) Selected%s\n\n", ColorGreen, ColorReset)
		return generator.AddressTypeLegacy
	case "3":
		fmt.Printf("    %s✓ Nested SegWit (P2SH) Selected%s\n\n", ColorGreen, ColorReset)
		return generator.AddressTypeNestedSegWit
	default:
		fmt.Printf("    %s✓ Taproot (P2TR) Selected%s\n\n", ColorGreen, ColorReset)
		return generator.AddressTypeTaproot
	}
}

// getBitcoinInput handles pattern input for Bitcoin addresses.
// For Taproot: lowercase only (Bech32m)
// For Legacy/SegWit: case-sensitive (Base58)
func getBitcoinInput(reader *bufio.Reader, addrType generator.AddressType) (string, string) {
	isBech32 := bitcoin.IsBech32Type(addrType)
	prefix := bitcoin.AddressPrefix(addrType)

	if isBech32 {
		// Taproot - Bech32m (lowercase only)
		fmt.Printf("    %sPrefix%s (after %s): ", ColorCyan, ColorReset, prefix)
		prefixInput, _ := reader.ReadString('\n')
		userPrefix := strings.TrimSpace(strings.ToLower(prefixInput))

		if userPrefix != "" && !bitcoin.IsValidPattern(userPrefix, addrType) {
			invalidChars := bitcoin.InvalidChars(userPrefix, addrType)
			fmt.Printf("    %s⚠ Invalid Bech32 character(s): %s%s\n", ColorRed, string(invalidChars), ColorReset)
			fmt.Printf("    %s  (Not allowed: 1, b, i, o)%s\n", ColorDim, ColorReset)
			userPrefix = ""
		}

		fmt.Printf("    %sSuffix%s (...): ", ColorCyan, ColorReset)
		suffixInput, _ := reader.ReadString('\n')
		userSuffix := strings.TrimSpace(strings.ToLower(suffixInput))

		if userSuffix != "" && !bitcoin.IsValidPattern(userSuffix, addrType) {
			invalidChars := bitcoin.InvalidChars(userSuffix, addrType)
			fmt.Printf("    %s⚠ Invalid Bech32 character(s): %s%s\n", ColorRed, string(invalidChars), ColorReset)
			fmt.Printf("    %s  (Not allowed: 1, b, i, o)%s\n", ColorDim, ColorReset)
			userSuffix = ""
		}

		return userPrefix, userSuffix
	} else {
		// Legacy/SegWit - Base58 (case-sensitive)
		fmt.Printf("    %sPrefix%s (after %s): ", ColorCyan, ColorReset, prefix)
		fmt.Printf("%s(Case-sensitive!)%s ", ColorYellow, ColorReset)
		prefixInput, _ := reader.ReadString('\n')
		userPrefix := strings.TrimSpace(prefixInput)

		if userPrefix != "" && !bitcoin.IsValidPattern(userPrefix, addrType) {
			invalidChars := bitcoin.InvalidChars(userPrefix, addrType)
			fmt.Printf("    %s⚠ Invalid Base58 character(s): %s%s\n", ColorRed, string(invalidChars), ColorReset)
			fmt.Printf("    %s  (Not allowed: 0, O, I, l)%s\n", ColorDim, ColorReset)
			userPrefix = ""
		}

		fmt.Printf("    %sSuffix%s (...): ", ColorCyan, ColorReset)
		suffixInput, _ := reader.ReadString('\n')
		userSuffix := strings.TrimSpace(suffixInput)

		if userSuffix != "" && !bitcoin.IsValidPattern(userSuffix, addrType) {
			invalidChars := bitcoin.InvalidChars(userSuffix, addrType)
			fmt.Printf("    %s⚠ Invalid Base58 character(s): %s%s\n", ColorRed, string(invalidChars), ColorReset)
			fmt.Printf("    %s  (Not allowed: 0, O, I, l)%s\n", ColorDim, ColorReset)
			userSuffix = ""
		}

		return userPrefix, userSuffix
	}
}

// getTronInput handles pattern input for Tron addresses.
// Tron addresses use Base58 encoding and always start with 'T'.
func getTronInput(reader *bufio.Reader) (string, string) {
	fmt.Printf("    %sPrefix%s (after T): ", ColorCyan, ColorReset)
	fmt.Printf("%s(Case-sensitive!)%s ", ColorYellow, ColorReset)
	prefixInput, _ := reader.ReadString('\n')
	prefix := strings.TrimSpace(prefixInput)

	// Special validation for Tron: first character after 'T' must be uppercase A-Z
	if prefix != "" {
		firstChar := prefix[0]
		isDigit := firstChar >= '0' && firstChar <= '9' // Check for any digit
		isLower := firstChar >= 'a' && firstChar <= 'z'

		if isDigit || isLower {
			fmt.Printf("    %s⚠ Invalid prefix! First character after 'T' MUST be an UPPERCASE letter (A-Z)%s\n", ColorRed, ColorReset)
			fmt.Printf("    %s  (Digits and lowercase letters are not possible at this position in Tron addresses)%s\n", ColorDim, ColorReset)
			prefix = ""
		}
	}

	if prefix != "" && !tron.IsValidBase58(prefix) {
		invalidChars := tron.InvalidBase58Chars(prefix)
		fmt.Printf("    %s⚠ Invalid Base58 character(s): %s%s\n", ColorRed, string(invalidChars), ColorReset)
		fmt.Printf("    %s  (Not allowed: 0, O, I, l)%s\n", ColorDim, ColorReset)
		prefix = ""
	}

	fmt.Printf("    %sSuffix%s (...): ", ColorCyan, ColorReset)
	suffixInput, _ := reader.ReadString('\n')
	suffix := strings.TrimSpace(suffixInput)

	if suffix != "" && !tron.IsValidBase58(suffix) {
		invalidChars := tron.InvalidBase58Chars(suffix)
		fmt.Printf("    %s⚠ Invalid Base58 character(s): %s%s\n", ColorRed, string(invalidChars), ColorReset)
		fmt.Printf("    %s  (Not allowed: 0, O, I, l)%s\n", ColorDim, ColorReset)
		suffix = ""
	}

	return prefix, suffix
}
