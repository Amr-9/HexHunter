// Aptos-specific kernel code
// This file is concatenated with ed25519_core.cl by kernel_builder.go

/* ========== SHA3-256 (Keccak with SHA3 padding) ========== */
/* Used for Aptos address derivation: SHA3-256(pubkey || 0x00) */

__constant uint64_t SHA3_RC[24] = {
    0x0000000000000001UL, 0x0000000000008082UL, 0x800000000000808aUL,
    0x8000000080008000UL, 0x000000000000808bUL, 0x0000000080000001UL,
    0x8000000080008081UL, 0x8000000000008009UL, 0x000000000000008aUL,
    0x0000000000000088UL, 0x0000000080008009UL, 0x000000008000000aUL,
    0x000000008000808bUL, 0x800000000000008bUL, 0x8000000000008089UL,
    0x8000000000008003UL, 0x8000000000008002UL, 0x8000000000000080UL,
    0x000000000000800aUL, 0x800000008000000aUL, 0x8000000080008081UL,
    0x8000000000008080UL, 0x0000000080000001UL, 0x8000000080008008UL
};

__constant int SHA3_ROTC[24] = {
    1, 3, 6, 10, 15, 21, 28, 36, 45, 55, 2, 14,
    27, 41, 56, 8, 25, 43, 62, 18, 39, 61, 20, 44
};

__constant int SHA3_PILN[24] = {
    10, 7, 11, 17, 18, 3, 5, 16, 8, 21, 24, 4,
    15, 23, 19, 13, 12, 2, 20, 14, 22, 9, 6, 1
};

inline uint64_t sha3_rotl64(uint64_t x, int y) {
    return (x << y) | (x >> (64 - y));
}

void sha3_keccakf(uint64_t st[25]) {
    uint64_t t, bc[5];

    for (int r = 0; r < 24; r++) {
        // Theta
        for (int i = 0; i < 5; i++)
            bc[i] = st[i] ^ st[i + 5] ^ st[i + 10] ^ st[i + 15] ^ st[i + 20];

        for (int i = 0; i < 5; i++) {
            t = bc[(i + 4) % 5] ^ sha3_rotl64(bc[(i + 1) % 5], 1);
            for (int j = 0; j < 25; j += 5)
                st[j + i] ^= t;
        }

        // Rho Pi
        t = st[1];
        for (int i = 0; i < 24; i++) {
            int j = SHA3_PILN[i];
            bc[0] = st[j];
            st[j] = sha3_rotl64(t, SHA3_ROTC[i]);
            t = bc[0];
        }

        // Chi
        for (int j = 0; j < 25; j += 5) {
            for (int i = 0; i < 5; i++)
                bc[i] = st[j + i];
            for (int i = 0; i < 5; i++)
                st[j + i] ^= (~bc[(i + 1) % 5]) & bc[(i + 2) % 5];
        }

        // Iota
        st[0] ^= SHA3_RC[r];
    }
}

// SHA3-256: Hash 33 bytes (pubkey || 0x00) -> 32 bytes output
void sha3_256(const uchar *input, int input_len, uchar *output) {
    uint64_t st[25];
    for (int i = 0; i < 25; i++) st[i] = 0;

    // Absorb input (little-endian)
    for (int i = 0; i < input_len; i++) {
        ((uchar*)st)[i] ^= input[i];
    }

    // SHA3 padding: 0x06 at end of message, 0x80 at end of rate block
    // For SHA3-256: rate = 136 bytes
    ((uchar*)st)[input_len] ^= 0x06;
    ((uchar*)st)[135] ^= 0x80;

    sha3_keccakf(st);

    // Extract 32 bytes output
    for (int i = 0; i < 32; i++) {
        output[i] = ((uchar*)st)[i];
    }
}

/* ========== Hex encoding ========== */
__constant uchar hex_chars[] = "0123456789abcdef";

inline void bytes_to_hex(const uchar *bytes, int len, uchar *hex_out) {
    for (int i = 0; i < len; i++) {
        hex_out[i * 2] = hex_chars[bytes[i] >> 4];
        hex_out[i * 2 + 1] = hex_chars[bytes[i] & 0x0F];
    }
}

__kernel void generate_aptos_address(
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

  // Prepare hash input: pubkey || 0x00 (single-sig scheme identifier)
  for (int i = 0; i < 32; i++) {
    hash_input[i] = public_key[i];
  }
  hash_input[32] = 0x00;

  // Compute SHA3-256
  sha3_256(hash_input, 33, address);

  // Convert to hex
  bytes_to_hex(address, 32, address_hex);

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
