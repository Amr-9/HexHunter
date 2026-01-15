package ui

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/ethvanity/pkg/generator"
	"github.com/ethvanity/pkg/generator/aptos"
	"github.com/ethvanity/pkg/generator/cpu"
	"github.com/ethvanity/pkg/generator/ethereum"
	"github.com/ethvanity/pkg/generator/solana"
)

// SelectEngineAndNetwork handles the engine (CPU/GPU) and network (Ethereum/Solana) selection.
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

	useGPU := engineChoice == "2" && gpuAvailable
	if useGPU {
		fmt.Printf("    %s✓ GPU Ready%s\n\n", ColorGreen, ColorReset)
	} else {
		fmt.Printf("    %s✓ CPU Ready%s\n\n", ColorGreen, ColorReset)
	}

	// Step 2: Select Network
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

	fmt.Printf("\n    %s→%s ", ColorGreen, ColorReset)
	networkChoice, _ := reader.ReadString('\n')
	networkChoice = strings.TrimSpace(networkChoice)

	// Step 3: Create the appropriate generator
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
	case generator.Aptos:
		return getAptosInput(reader)
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

// AskToContinue prompts user to continue or exit
func AskToContinue() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n    %s[Enter]%s Continue searching  │  %s[Q]%s Exit\n", ColorGreen, ColorReset, ColorRed, ColorReset)
	fmt.Printf("    %s→%s ", ColorCyan, ColorReset)
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
