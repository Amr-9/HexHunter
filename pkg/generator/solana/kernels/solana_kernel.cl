// Solana-specific kernel code
// This file is concatenated with ed25519_core.cl by kernel_builder.go

inline __attribute__((always_inline))
static uchar * base58_encode(uchar *in, size_t *out_len, uchar *out) {
  unsigned int binary[8];

  #pragma unroll
  for (int i = 0; i < 8; ++i) {
    binary[i] = as_uint(((uchar4*)in)[i].wzyx);
  }

  unsigned int in_leading_0s = (clz(binary[0]) + (binary[0] == 0) * clz(binary[1])) >> 3;
  if (in_leading_0s == 8) {
    for (; in_leading_0s < 32; in_leading_0s++) if (in[in_leading_0s]) break;
  }

  uint64_t intermediate[9] = {0};

  intermediate[1] += (uint64_t) binary[0] * (uint64_t) 513735UL;
  intermediate[2] += (uint64_t) binary[0] * (uint64_t) 77223048UL;
  intermediate[3] += (uint64_t) binary[0] * (uint64_t) 437087610UL;
  intermediate[4] += (uint64_t) binary[0] * (uint64_t) 300156666UL;
  intermediate[5] += (uint64_t) binary[0] * (uint64_t) 605448490UL;
  intermediate[6] += (uint64_t) binary[0] * (uint64_t) 214625350UL;
  intermediate[7] += (uint64_t) binary[0] * (uint64_t) 141436834UL;
  intermediate[8] += (uint64_t) binary[0] * (uint64_t) 379377856UL;
  intermediate[2] += (uint64_t) binary[1] * (uint64_t) 78508UL;
  intermediate[3] += (uint64_t) binary[1] * (uint64_t) 646269101UL;
  intermediate[4] += (uint64_t) binary[1] * (uint64_t) 118408823UL;
  intermediate[5] += (uint64_t) binary[1] * (uint64_t) 91512303UL;
  intermediate[6] += (uint64_t) binary[1] * (uint64_t) 209184527UL;
  intermediate[7] += (uint64_t) binary[1] * (uint64_t) 413102373UL;
  intermediate[8] += (uint64_t) binary[1] * (uint64_t) 153715680UL;
  intermediate[3] += (uint64_t) binary[2] * (uint64_t) 11997UL;
  intermediate[4] += (uint64_t) binary[2] * (uint64_t) 486083817UL;
  intermediate[5] += (uint64_t) binary[2] * (uint64_t) 3737691UL;
  intermediate[6] += (uint64_t) binary[2] * (uint64_t) 294005210UL;
  intermediate[7] += (uint64_t) binary[2] * (uint64_t) 247894721UL;
  intermediate[8] += (uint64_t) binary[2] * (uint64_t) 289024608UL;
  intermediate[4] += (uint64_t) binary[3] * (uint64_t) 1833UL;
  intermediate[5] += (uint64_t) binary[3] * (uint64_t) 324463681UL;
  intermediate[6] += (uint64_t) binary[3] * (uint64_t) 385795061UL;
  intermediate[7] += (uint64_t) binary[3] * (uint64_t) 551597588UL;
  intermediate[8] += (uint64_t) binary[3] * (uint64_t) 21339008UL;
  intermediate[5] += (uint64_t) binary[4] * (uint64_t) 280UL;
  intermediate[6] += (uint64_t) binary[4] * (uint64_t) 127692781UL;
  intermediate[7] += (uint64_t) binary[4] * (uint64_t) 389432875UL;
  intermediate[8] += (uint64_t) binary[4] * (uint64_t) 357132832UL;
  intermediate[6] += (uint64_t) binary[5] * (uint64_t) 42UL;
  intermediate[7] += (uint64_t) binary[5] * (uint64_t) 537767569UL;
  intermediate[8] += (uint64_t) binary[5] * (uint64_t) 410450016UL;
  intermediate[7] += (uint64_t) binary[6] * (uint64_t) 6UL;
  intermediate[8] += (uint64_t) binary[6] * (uint64_t) 356826688UL;
  intermediate[8] += (uint64_t) binary[7] * (uint64_t) 1UL;

  intermediate[7] += intermediate[8] / 656356768UL;
  intermediate[8] %= 656356768UL;
  intermediate[6] += intermediate[7] / 656356768UL;
  intermediate[7] %= 656356768UL;
  intermediate[5] += intermediate[6] / 656356768UL;
  intermediate[6] %= 656356768UL;
  intermediate[4] += intermediate[5] / 656356768UL;
  intermediate[5] %= 656356768UL;
  intermediate[3] += intermediate[4] / 656356768UL;
  intermediate[4] %= 656356768UL;
  intermediate[2] += intermediate[3] / 656356768UL;
  intermediate[3] %= 656356768UL;
  intermediate[1] += intermediate[2] / 656356768UL;
  intermediate[2] %= 656356768UL;
  intermediate[0] += intermediate[1] / 656356768UL;
  intermediate[1] %= 656356768UL;

  #pragma unroll
  for (int i = 0; i < 9; i++) {
    out[5 * i + 4] = ((((unsigned int) intermediate[i]) / 1U       ) % 58U);
    out[5 * i + 3] = ((((unsigned int) intermediate[i]) / 58U      ) % 58U);
    out[5 * i + 2] = ((((unsigned int) intermediate[i]) / 3364U    ) % 58U);
    out[5 * i + 1] = ((((unsigned int) intermediate[i]) / 195112U  ) % 58U);
    out[5 * i + 0] = ( ((unsigned int) intermediate[i]) / 11316496U);
  }

  unsigned int t = as_uint(((uchar4 *)out)[0].wzyx);
  unsigned int raw_leading_0s = (clz(t) + (t == 0) * clz(as_uint(((uchar4 *)out)[1].wzyx))) >> 3;
  if (raw_leading_0s == 8) {
    for (; raw_leading_0s < 45; raw_leading_0s++) if (out[raw_leading_0s]) break;
  }

  unsigned int skip = (raw_leading_0s - in_leading_0s) * (raw_leading_0s > in_leading_0s);
  *out_len = (9 * 5) - skip;
  return out + skip;
}

__kernel void generate_pubkey(
    constant uchar *seed,
    global uchar *out,
    global uchar *occupied_bytes,
    global uchar *group_offset,
    constant uchar *prefix,
    constant uchar *suffix,
    const uint prefix_len,
    const uint suffix_len,
    const uint case_sensitive
) {
  uchar public_key[32] __attribute__((aligned(4)));
  uchar private_key[64];
  uchar key_base[32];

  #pragma unroll
  for (size_t i = 0; i < 32; i++) {
    key_base[i] = seed[i];
  }
  const int global_id = (*group_offset) * get_global_size(0) + get_global_id(0);

  for (size_t i = 0; i < *occupied_bytes; i++) {
    key_base[31 - i] += ((global_id >> (i * 8)) & 0xFF);
  }

  ed25519_create_keypair(public_key, private_key, key_base);
  size_t length;
  uchar addr_buffer[45] __attribute__((aligned(4)));
  uchar *addr_raw = base58_encode(public_key, &length, addr_buffer);

  constant uchar base58_alphabet[] = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";

  unsigned int match = 1;

  for (uint i = 0; i < prefix_len && match; i++) {
    uchar addr_char = base58_alphabet[addr_raw[i]];
    uchar prefix_char = prefix[i];
    
    if (!case_sensitive) {
      addr_char = to_lower_char(addr_char);
      prefix_char = to_lower_char(prefix_char);
    }
    
    if (addr_char != prefix_char) {
      match = 0;
    }
  }

  for (uint i = 0; i < suffix_len && match; i++) {
    uchar addr_char = base58_alphabet[addr_raw[length - suffix_len + i]];
    uchar suffix_char = suffix[i];
    
    if (!case_sensitive) {
      addr_char = to_lower_char(addr_char);
      suffix_char = to_lower_char(suffix_char);
    }
    
    if (addr_char != suffix_char) {
      match = 0;
    }
  }

  if (match) {
    if (out[0] == 0) {
      out[0] = length;
      for (size_t j = 0; j < 32; j++) {
        out[j + 1] = key_base[j];
      }
    }
    if (length < out[0]) {
      out[0] = length;
      for (size_t j = 0; j < 32; j++) {
        out[j + 1] = key_base[j];
      }
    }
  }
}
