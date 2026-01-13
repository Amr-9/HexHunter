package generator

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

// CPUGenerator implements the Generator interface using CPU-based goroutines.
// It uses a worker pool pattern to maximize CPU utilization.
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
func (g *CPUGenerator) Stats() Stats {
	attempts := atomic.LoadUint64(&g.attempts)
	elapsed := time.Since(g.startTime).Seconds()

	var hashRate float64
	if elapsed > 0 {
		hashRate = float64(attempts) / elapsed
	}

	return Stats{
		Attempts:    attempts,
		HashRate:    hashRate,
		ElapsedSecs: elapsed,
	}
}

// Start begins the vanity address search with the given configuration.
func (g *CPUGenerator) Start(ctx context.Context, config *Config) (<-chan Result, error) {
	resultChan := make(chan Result, 1)
	g.startTime = time.Now()
	atomic.StoreUint64(&g.attempts, 0)

	// Create the matcher with pre-processed patterns
	matcher := NewMatcher(config.Prefix, config.Suffix)

	// Create a done channel to signal all workers to stop
	done := make(chan struct{})
	var closeOnce sync.Once // Ensures done is closed exactly once

	// Start worker goroutines
	workers := g.workers
	if config.Workers > 0 {
		workers = config.Workers
	}

	for i := 0; i < workers; i++ {
		go g.worker(ctx, matcher, resultChan, done, &closeOnce)
	}

	return resultChan, nil
}

// worker is a single worker goroutine that continuously generates addresses.
func (g *CPUGenerator) worker(ctx context.Context, matcher *Matcher, resultChan chan<- Result, done chan struct{}, closeOnce *sync.Once) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		default:
			// Generate a new private key
			privateKey, err := crypto.GenerateKey()
			if err != nil {
				continue // Rare error, just retry
			}

			// Increment attempt counter (lock-free)
			atomic.AddUint64(&g.attempts, 1)

			// Get the address bytes directly (no string conversion)
			address := crypto.PubkeyToAddress(privateKey.PublicKey)

			// Check if address matches the criteria
			if matcher.Matches(address.Bytes()) {
				// Found a match! Send result and signal other workers to stop
				result := Result{
					Address:    address.Hex(),
					PrivateKey: privateKeyToHex(privateKey),
				}

				select {
				case resultChan <- result:
					// Safely close done channel exactly once
					closeOnce.Do(func() { close(done) })
				default:
					// Result already sent by another worker
				}
				return
			}
		}
	}
}

// privateKeyToHex converts a private key to its hex representation.
func privateKeyToHex(privateKey *ecdsa.PrivateKey) string {
	return hex.EncodeToString(crypto.FromECDSA(privateKey))
}
