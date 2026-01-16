// Sui-specific kernel code
// This file is concatenated with ed25519_core.cl by kernel_builder.go
// Implements Blake2b-256 for Sui address derivation (RFC 7693 compliant)

/* ========== Blake2b-256 Implementation ========== */
/* Used for Sui address derivation: Blake2b-256(0x00 || pubkey) */

// Blake2b IV (initialization vector)
__constant uint64_t BLAKE2B_IV[8] = {
    0x6a09e667f3bcc908UL, 0xbb67ae8584caa73bUL,
    0x3c6ef372fe94f82bUL, 0xa54ff53a5f1d36f1UL,
    0x510e527fade682d1UL, 0x9b05688c2b3e6c1fUL,
    0x1f83d9abfb41bd6bUL, 0x5be0cd19137e2179UL
};

// Blake2b sigma permutation table
__constant uchar BLAKE2B_SIGMA[12][16] = {
    {  0,  1,  2,  3,  4,  5,  6,  7,  8,  9, 10, 11, 12, 13, 14, 15 },
    { 14, 10,  4,  8,  9, 15, 13,  6,  1, 12,  0,  2, 11,  7,  5,  3 },
    { 11,  8, 12,  0,  5,  2, 15, 13, 10, 14,  3,  6,  7,  1,  9,  4 },
    {  7,  9,  3,  1, 13, 12, 11, 14,  2,  6,  5, 10,  4,  0, 15,  8 },
    {  9,  0,  5,  7,  2,  4, 10, 15, 14,  1, 11, 12,  6,  8,  3, 13 },
    {  2, 12,  6, 10,  0, 11,  8,  3,  4, 13,  7,  5, 15, 14,  1,  9 },
    { 12,  5,  1, 15, 14, 13,  4, 10,  0,  7,  6,  3,  9,  2,  8, 11 },
    { 13, 11,  7, 14, 12,  1,  3,  9,  5,  0, 15,  4,  8,  6,  2, 10 },
    {  6, 15, 14,  9, 11,  3,  0,  8, 12,  2, 13,  7,  1,  4, 10,  5 },
    { 10,  2,  8,  4,  7,  6,  1,  5, 15, 11,  9, 14,  3, 12, 13,  0 },
    {  0,  1,  2,  3,  4,  5,  6,  7,  8,  9, 10, 11, 12, 13, 14, 15 },
    { 14, 10,  4,  8,  9, 15, 13,  6,  1, 12,  0,  2, 11,  7,  5,  3 }
};

inline uint64_t blake2b_rotr64(uint64_t x, int n) {
    return (x >> n) | (x << (64 - n));
}

// Blake2b G mixing function
inline void blake2b_G(uint64_t *v, int a, int b, int c, int d, uint64_t x, uint64_t y) {
    v[a] = v[a] + v[b] + x;
    v[d] = blake2b_rotr64(v[d] ^ v[a], 32);
    v[c] = v[c] + v[d];
    v[b] = blake2b_rotr64(v[b] ^ v[c], 24);
    v[a] = v[a] + v[b] + y;
    v[d] = blake2b_rotr64(v[d] ^ v[a], 16);
    v[c] = v[c] + v[d];
    v[b] = blake2b_rotr64(v[b] ^ v[c], 63);
}

// Blake2b compression function
void blake2b_compress(uint64_t h[8], const uchar block[128], uint64_t t, int last) {
    uint64_t v[16];
    uint64_t m[16];
    
    // Load message block (little-endian)
    for (int i = 0; i < 16; i++) {
        m[i] = ((uint64_t)block[i * 8 + 0]) |
               ((uint64_t)block[i * 8 + 1] << 8) |
               ((uint64_t)block[i * 8 + 2] << 16) |
               ((uint64_t)block[i * 8 + 3] << 24) |
               ((uint64_t)block[i * 8 + 4] << 32) |
               ((uint64_t)block[i * 8 + 5] << 40) |
               ((uint64_t)block[i * 8 + 6] << 48) |
               ((uint64_t)block[i * 8 + 7] << 56);
    }
    
    // Initialize working vector
    for (int i = 0; i < 8; i++) {
        v[i] = h[i];
        v[i + 8] = BLAKE2B_IV[i];
    }
    
    v[12] ^= t;           // Low 64 bits of offset
    v[13] ^= 0;           // High 64 bits of offset (always 0 for our case)
    if (last) v[14] = ~v[14];  // Last block flag
    
    // 12 rounds of mixing
    for (int r = 0; r < 12; r++) {
        blake2b_G(v, 0, 4,  8, 12, m[BLAKE2B_SIGMA[r][ 0]], m[BLAKE2B_SIGMA[r][ 1]]);
        blake2b_G(v, 1, 5,  9, 13, m[BLAKE2B_SIGMA[r][ 2]], m[BLAKE2B_SIGMA[r][ 3]]);
        blake2b_G(v, 2, 6, 10, 14, m[BLAKE2B_SIGMA[r][ 4]], m[BLAKE2B_SIGMA[r][ 5]]);
        blake2b_G(v, 3, 7, 11, 15, m[BLAKE2B_SIGMA[r][ 6]], m[BLAKE2B_SIGMA[r][ 7]]);
        blake2b_G(v, 0, 5, 10, 15, m[BLAKE2B_SIGMA[r][ 8]], m[BLAKE2B_SIGMA[r][ 9]]);
        blake2b_G(v, 1, 6, 11, 12, m[BLAKE2B_SIGMA[r][10]], m[BLAKE2B_SIGMA[r][11]]);
        blake2b_G(v, 2, 7,  8, 13, m[BLAKE2B_SIGMA[r][12]], m[BLAKE2B_SIGMA[r][13]]);
        blake2b_G(v, 3, 4,  9, 14, m[BLAKE2B_SIGMA[r][14]], m[BLAKE2B_SIGMA[r][15]]);
    }
    
    // Finalize
    for (int i = 0; i < 8; i++) {
        h[i] ^= v[i] ^ v[i + 8];
    }
}

// Blake2b-256: Hash input -> 32 bytes output
// Optimized for small inputs (33 bytes for Sui: 0x00 || pubkey)
void blake2b_256(const uchar *input, int input_len, uchar *output) {
    uint64_t h[8];
    uchar block[128];
    
    // Initialize state with IV
    for (int i = 0; i < 8; i++) {
        h[i] = BLAKE2B_IV[i];
    }
    
    // Parameter block: digest length = 32, key length = 0, fanout = 1, depth = 1
    h[0] ^= 0x01010020;  // 0x01010000 | (key_len << 8) | digest_len
    
    // Prepare padded block (for inputs <= 128 bytes, we only need one block)
    for (int i = 0; i < 128; i++) {
        block[i] = (i < input_len) ? input[i] : 0;
    }
    
    // Compress (single block, last block)
    blake2b_compress(h, block, input_len, 1);
    
    // Extract output (first 32 bytes, little-endian)
    for (int i = 0; i < 4; i++) {
        for (int j = 0; j < 8; j++) {
            output[i * 8 + j] = (h[i] >> (j * 8)) & 0xFF;
        }
    }
}

/* ========== Hex encoding ========== */
__constant uchar sui_hex_chars[] = "0123456789abcdef";

inline void sui_bytes_to_hex(const uchar *bytes, int len, uchar *hex_out) {
    for (int i = 0; i < len; i++) {
        hex_out[i * 2] = sui_hex_chars[bytes[i] >> 4];
        hex_out[i * 2 + 1] = sui_hex_chars[bytes[i] & 0x0F];
    }
}

/* ========== Sui Address Generation Kernel ========== */
__kernel void generate_sui_address(
    constant uchar *seed,
    global uchar *out,
    global uchar *occupied_bytes,
    global uchar *group_offset,
    constant uchar *prefix,
    constant uchar *suffix,
    constant uchar *contains,
    const uint prefix_len,
    const uint suffix_len,
    const uint contains_len,
    const uint case_sensitive
) {
    uchar public_key[32] __attribute__((aligned(4)));
    uchar private_key[64];
    uchar key_base[32];
    uchar hash_input[33];
    uchar address[32];
    uchar address_hex[64];

    #pragma unroll
    for (size_t i = 0; i < 32; i++) {
        key_base[i] = seed[i];
    }
    const int global_id = (*group_offset) * get_global_size(0) + get_global_id(0);

    for (size_t i = 0; i < *occupied_bytes; i++) {
        key_base[31 - i] += ((global_id >> (i * 8)) & 0xFF);
    }

    // Generate Ed25519 keypair
    ed25519_create_keypair(public_key, private_key, key_base);

    // Prepare hash input: 0x00 || pubkey (Ed25519 scheme flag FIRST for Sui)
    hash_input[0] = 0x00;
    for (int i = 0; i < 32; i++) {
        hash_input[i + 1] = public_key[i];
    }

    // Compute Blake2b-256
    blake2b_256(hash_input, 33, address);

    // Convert to hex
    sui_bytes_to_hex(address, 32, address_hex);

    // Pattern matching
    unsigned int match = 1;

    // Check prefix
    for (uint i = 0; i < prefix_len && match; i++) {
        uchar addr_char = address_hex[i];
        uchar prefix_char = prefix[i];
        
        if (!case_sensitive) {
            addr_char = to_lower_char(addr_char);
            prefix_char = to_lower_char(prefix_char);
        }
        
        if (addr_char != prefix_char) {
            match = 0;
        }
    }

    // Check suffix
    for (uint i = 0; i < suffix_len && match; i++) {
        uchar addr_char = address_hex[64 - suffix_len + i];
        uchar suffix_char = suffix[i];
        
        if (!case_sensitive) {
            addr_char = to_lower_char(addr_char);
            suffix_char = to_lower_char(suffix_char);
        }
        
        if (addr_char != suffix_char) {
            match = 0;
        }
    }

    // Check contains - search for pattern in middle section
    if (match && contains_len > 0) {
        uint start_pos = prefix_len;
        uint end_pos = 64 - suffix_len;
        
        if (end_pos <= start_pos || contains_len > end_pos - start_pos) {
            match = 0;
        } else {
            unsigned int contains_found = 0;
            
            // Sliding window search through middle section
            for (uint pos = start_pos; pos <= end_pos - contains_len && !contains_found; pos++) {
                unsigned int pos_match = 1;
                
                for (uint i = 0; i < contains_len && pos_match; i++) {
                    uchar addr_char = address_hex[pos + i];
                    uchar contains_char = contains[i];
                    
                    if (!case_sensitive) {
                        addr_char = to_lower_char(addr_char);
                        contains_char = to_lower_char(contains_char);
                    }
                    
                    if (addr_char != contains_char) {
                        pos_match = 0;
                    }
                }
                
                if (pos_match) {
                    contains_found = 1;
                }
            }
            
            if (!contains_found) {
                match = 0;
            }
        }
    }

    if (match) {
        if (out[0] == 0) {
            out[0] = 64;
            for (size_t j = 0; j < 32; j++) {
                out[j + 1] = key_base[j];
            }
        }
    }
}
