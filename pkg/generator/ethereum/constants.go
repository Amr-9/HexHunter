package ethereum

const (
	// Batch size = Table size (2^20 = 1,048,576)
	globalWorkSize = 1 << 20
	localWorkSize  = 256
	outputSize     = 64                  // Only single result now
	tableSize      = globalWorkSize * 64 // 64 bytes per point (Affine)
)
