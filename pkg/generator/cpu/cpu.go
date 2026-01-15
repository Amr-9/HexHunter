package cpu

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethvanity/pkg/generator"
	"github.com/ethvanity/pkg/generator/aptos"
	"github.com/ethvanity/pkg/generator/ethereum"
	"github.com/ethvanity/pkg/generator/solana"
	"github.com/mr-tron/base58"
)

// CPUGenerator implements the Generator interface using CPU-based goroutines.
// It supports both Ethereum and Solana networks.
type CPUGenerator struct {
	attempts  uint64    // Atomic counter for total attempts
	startTime time.Time // When generation started
	workers   int       // Number of concurrent workers
}

// NewCPUGenerator creates a new CPU-based generator.
// If workers is 0, it defaults to the number of CPU cores.
func NewCPUGenerator(workers int) *CPUGenerator {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	return &CPUGenerator{
		workers: workers,
	}
}

// Name returns the implementation name.
func (g *CPUGenerator) Name() string {
	return "CPU"
}

// Stats returns the current performance statistics.
func (g *CPUGenerator) Stats() generator.Stats {
	attempts := atomic.LoadUint64(&g.attempts)
	elapsed := time.Since(g.startTime).Seconds()

	var hashRate float64
	if elapsed > 0 {
		hashRate = float64(attempts) / elapsed
	}

	return generator.Stats{
		Attempts:    attempts,
		HashRate:    hashRate,
		ElapsedSecs: elapsed,
	}
}

// Start begins the vanity address search with the given configuration.
func (g *CPUGenerator) Start(ctx context.Context, config *generator.Config) (<-chan generator.Result, error) {
	resultChan := make(chan generator.Result, 1)
	g.startTime = time.Now()
	atomic.StoreUint64(&g.attempts, 0)

	done := make(chan struct{})
	var closeOnce sync.Once

	workers := g.workers
	if config.Workers > 0 {
		workers = config.Workers
	}

	// Route to appropriate worker based on network
	switch config.Network {
	case generator.Solana:
		matcher := solana.NewSolanaMatcher(config.Prefix, config.Suffix)
		for i := 0; i < workers; i++ {
			go g.workerSolana(ctx, matcher, resultChan, done, &closeOnce)
		}
	case generator.Aptos:
		matcher := aptos.NewAptosMatcher(config.Prefix, config.Suffix)
		for i := 0; i < workers; i++ {
			go g.workerAptos(ctx, matcher, resultChan, done, &closeOnce)
		}
	default: // Ethereum
		matcher := ethereum.NewMatcher(config.Prefix, config.Suffix)
		for i := 0; i < workers; i++ {
			go g.workerEthereum(ctx, matcher, resultChan, done, &closeOnce)
		}
	}

	return resultChan, nil
}

// workerEthereum generates Ethereum addresses (secp256k1 + Keccak-256)
func (g *CPUGenerator) workerEthereum(ctx context.Context, matcher *ethereum.Matcher, resultChan chan<- generator.Result, done chan struct{}, closeOnce *sync.Once) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		default:
			privateKey, err := crypto.GenerateKey()
			if err != nil {
				continue
			}

			atomic.AddUint64(&g.attempts, 1)

			address := crypto.PubkeyToAddress(privateKey.PublicKey)

			if matcher.Matches(address.Bytes()) {
				result := generator.Result{
					Network:    generator.Ethereum,
					Address:    address.Hex(),
					PrivateKey: privateKeyToHex(privateKey),
				}

				select {
				case resultChan <- result:
					closeOnce.Do(func() { close(done) })
				default:
				}
				return
			}
		}
	}
}

// workerSolana generates Solana addresses (Ed25519 + Base58)
func (g *CPUGenerator) workerSolana(ctx context.Context, matcher *solana.SolanaMatcher, resultChan chan<- generator.Result, done chan struct{}, closeOnce *sync.Once) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		default:
			pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				continue
			}

			atomic.AddUint64(&g.attempts, 1)

			// Solana address is the Base58-encoded public key
			address := base58.Encode(pubKey)

			if matcher.Matches(address) {
				// Solana uses 64-byte keypair (seed + pubkey)
				result := generator.Result{
					Network:    generator.Solana,
					Address:    address,
					PrivateKey: base58.Encode(privKey),
				}

				select {
				case resultChan <- result:
					closeOnce.Do(func() { close(done) })
				default:
				}
				return
			}
		}
	}
}

// workerAptos generates Aptos addresses (Ed25519 + SHA3-256)
func (g *CPUGenerator) workerAptos(ctx context.Context, matcher *aptos.AptosMatcher, resultChan chan<- generator.Result, done chan struct{}, closeOnce *sync.Once) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		default:
			pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				continue
			}

			atomic.AddUint64(&g.attempts, 1)

			// Aptos address = SHA3-256(pubkey || 0x00)
			address := aptos.DeriveAddress(pubKey)

			if matcher.Matches(address) {
				// Return the seed (first 32 bytes of privKey) as hex
				result := generator.Result{
					Network:    generator.Aptos,
					Address:    address,
					PrivateKey: hex.EncodeToString(privKey.Seed()),
				}

				select {
				case resultChan <- result:
					closeOnce.Do(func() { close(done) })
				default:
				}
				return
			}
		}
	}
}

// privateKeyToHex converts an Ethereum private key to its hex representation.
func privateKeyToHex(privateKey *ecdsa.PrivateKey) string {
	return hex.EncodeToString(crypto.FromECDSA(privateKey))
}
