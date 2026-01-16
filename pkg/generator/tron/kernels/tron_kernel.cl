/* tron_kernel.cl - Optimized Tron Address Generation Kernel
 * 
 * This kernel performs:
 * 1. secp256k1 point addition using precomputed table
 * 2. Keccak-256 hash of public key
 * 3. Base58Check encoding (0x41 prefix + address + checksum)
 * 4. In-kernel pattern matching (prefix/suffix)
 * 
 * Only returns when a match is found, eliminating 20MB transfers!
 */

#pragma OPENCL EXTENSION cl_khr_global_int32_base_atomics : enable

typedef struct { uint w[8]; } uint256;
typedef struct { uint256 x; uint256 y; uint256 z; } point_j;
typedef struct { uint256 x; uint256 y; } point_a;

#define WORKGROUP_SIZE 256

/* Constants */
__constant uint256 P_CONST = {{ 0xFFFFFC2F, 0xFFFFFFFE, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF }};
__constant uint256 ONE = {{ 1, 0, 0, 0, 0, 0, 0, 0 }};

/* Base58 alphabet */
__constant char BASE58_ALPHABET[] = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";

/* Keccak Tables */
__constant ulong KECCAK_RC[24] = { 0x0000000000000001UL, 0x0000000000008082UL, 0x800000000000808aUL, 0x8000000080008000UL, 0x000000000000808bUL, 0x0000000080000001UL, 0x8000000080008081UL, 0x8000000000008009UL, 0x000000000000008aUL, 0x0000000000000088UL, 0x0000000080008009UL, 0x000000008000000aUL, 0x000000008000808bUL, 0x800000000000008bUL, 0x8000000000008089UL, 0x8000000000008003UL, 0x8000000000008002UL, 0x8000000000000080UL, 0x000000000000800aUL, 0x800000008000000aUL, 0x8000000080008081UL, 0x8000000000008080UL, 0x0000000080000001UL, 0x8000000080008008UL };
__constant int KECCAK_ROT[24] = { 1, 3, 6, 10, 15, 21, 28, 36, 45, 55, 2, 14, 27, 41, 56, 8, 25, 43, 62, 18, 39, 61, 20, 44 };
__constant int KECCAK_PI[24] = { 10, 7, 11, 17, 18, 3, 5, 16, 8, 21, 24, 4, 15, 23, 19, 13, 12, 2, 20, 14, 22, 9, 6, 1 };

/* SHA256 constants */
__constant uint SHA256_K[64] = {
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2
};

/* === Math Functions === */
void load_const(uint256 *d, __constant const uint256 *s) { for(int i=0;i<8;i++) d->w[i]=s->w[i]; }
int is_zero(const uint256 *a) { for(int i=0;i<8;i++) if(a->w[i]!=0) return 0; return 1; }
void set_u64(uint256 *a, ulong v) { a->w[0]=(uint)v; a->w[1]=(uint)(v>>32); for(int i=2;i<8;i++) a->w[i]=0; }
void uint256_copy(uint256 *d, const uint256 *s) { for(int i=0;i<8;i++) d->w[i]=s->w[i]; }
void uint256_copy_local(__local uint256 *d, const uint256 *s) { for(int i=0;i<8;i++) d->w[i]=s->w[i]; }
void uint256_copy_from_local(uint256 *d, __local const uint256 *s) { for(int i=0;i<8;i++) d->w[i]=s->w[i]; }

uint add_c(uint256 *r, const uint256 *a, const uint256 *b) {
    ulong c=0;
    for(int i=0;i<8;i++) { ulong s=(ulong)a->w[i]+b->w[i]+c; r->w[i]=(uint)s; c=s>>32; }
    return (uint)c;
}
uint sub_b(uint256 *r, const uint256 *a, const uint256 *b) {
    long br=0;
    for(int i=0;i<8;i++) { long d=(long)a->w[i]-b->w[i]-br; r->w[i]=(uint)d; br=(d<0)?1:0; }
    return (uint)br;
}
int gte(const uint256 *a, const uint256 *b) {
    for(int i=7;i>=0;i--) { if(a->w[i]>b->w[i]) return 1; if(a->w[i]<b->w[i]) return 0; }
    return 1;
}

void mod_add(uint256 *r, const uint256 *a, const uint256 *b) {
    uint256 P; load_const(&P, &P_CONST);
    if(add_c(r,a,b) || gte(r,&P)) sub_b(r,r,&P);
}
void mod_sub(uint256 *r, const uint256 *a, const uint256 *b) {
    uint256 P; load_const(&P, &P_CONST);
    if(sub_b(r,a,b)) add_c(r,r,&P);
}

void mod_mul(uint256 *result, const uint256 *a, const uint256 *b) {
    ulong u[16] = {0};
    for (int i = 0; i < 8; i++) {
        ulong carry = 0;
        for (int j = 0; j < 8; j++) {
            ulong prod = (ulong)a->w[i] * (ulong)b->w[j] + u[i+j] + carry;
            u[i+j] = prod & 0xFFFFFFFF;
            carry = prod >> 32;
        }
        u[i+8] = carry;
    }
    for (int iter = 0; iter < 6; iter++) {
        int high_zero = 1;
        for(int k=8; k<16; k++) if(u[k] != 0) high_zero = 0;
        if(high_zero) break;
        uint high[8];
        for(int k=0; k<8; k++) { high[k] = (uint)u[k+8]; u[k+8] = 0; }
        ulong carry_mul = 0;
        for(int k=0; k<8; k++) {
            ulong term = (ulong)high[k] * 977UL + u[k] + carry_mul;
            u[k] = term & 0xFFFFFFFF;
            carry_mul = term >> 32;
        }
        u[8] += (uint)carry_mul;
        ulong carry_shift = 0;
        for(int k=0; k<8; k++) {
             ulong term = (ulong)high[k] + u[k+1] + carry_shift;
             u[k+1] = term & 0xFFFFFFFF;
             carry_shift = term >> 32;
        }
        int idx = 9;
        while(carry_shift > 0 && idx < 16) {
            ulong term = u[idx] + carry_shift;
            u[idx] = term & 0xFFFFFFFF;
            carry_shift = term >> 32;
            idx++;
        }
    }
    uint256 final_res;
    for(int i=0; i<8; i++) final_res.w[i] = (uint)u[i];
    uint256 P; load_const(&P, &P_CONST);
    while (gte(&final_res, &P)) sub_b(&final_res, &final_res, &P);
    *result = final_res;
}

void mod_pow(uint256 *result, const uint256 *base, const uint256 *exp) {
    uint256 r; set_u64(&r, 1);
    uint256 b; uint256_copy(&b, base);
    for (int i = 0; i < 8; i++) {
        uint w = exp->w[i];
        for (int j = 0; j < 32; j++) {
            if ((w >> j) & 1) mod_mul(&r, &r, &b);
            mod_mul(&b, &b, &b);
        }
    }
    *result = r;
}

void mod_inv(uint256 *result, const uint256 *a) {
    uint256 P_minus_2; load_const(&P_minus_2, &P_CONST);
    uint256 two; set_u64(&two, 2);
    sub_b(&P_minus_2, &P_minus_2, &two);
    mod_pow(result, a, &P_minus_2);
}

/* === Mixed Point Addition === */
void j_add_mixed(point_j *r, const point_j *p, const point_a *q) {
    if(is_zero(&p->z)) {
        uint256_copy(&r->x, &q->x);
        uint256_copy(&r->y, &q->y);
        set_u64(&r->z, 1);
        return;
    }
    if(is_zero(&q->x) && is_zero(&q->y)) { *r = *p; return; }
    
    uint256 rx, ry, rz, Z1Z1, U2, S2, H, HH, I, J, r_val, V, tmp;
    mod_mul(&Z1Z1, &p->z, &p->z);
    mod_mul(&U2, &q->x, &Z1Z1);
    mod_mul(&S2, &q->y, &p->z);
    mod_mul(&S2, &S2, &Z1Z1);
    mod_sub(&H, &U2, &p->x);
    mod_mul(&HH, &H, &H);
    mod_add(&I, &HH, &HH); mod_add(&I, &I, &I);
    mod_mul(&J, &H, &I);
    mod_sub(&r_val, &S2, &p->y); mod_add(&r_val, &r_val, &r_val);
    mod_mul(&V, &p->x, &I);
    mod_mul(&rx, &r_val, &r_val);
    mod_sub(&rx, &rx, &J); mod_sub(&rx, &rx, &V); mod_sub(&rx, &rx, &V);
    mod_sub(&tmp, &V, &rx);
    mod_mul(&ry, &r_val, &tmp);
    mod_mul(&tmp, &p->y, &J); mod_add(&tmp, &tmp, &tmp);
    mod_sub(&ry, &ry, &tmp);
    mod_mul(&rz, &p->z, &H); mod_add(&rz, &rz, &rz);
    r->x = rx; r->y = ry; r->z = rz;
}

/* === Keccak-256 === */
ulong rotate(ulong x, int s) { return (x<<s)|(x>>(64-s)); }
void keccak_f1600(ulong *st) {
    ulong bc[5], t;
    for(int r=0; r<24; r++) {
        bc[0]=st[0]^st[5]^st[10]^st[15]^st[20];
        bc[1]=st[1]^st[6]^st[11]^st[16]^st[21];
        bc[2]=st[2]^st[7]^st[12]^st[17]^st[22];
        bc[3]=st[3]^st[8]^st[13]^st[18]^st[23];
        bc[4]=st[4]^st[9]^st[14]^st[19]^st[24];
        for(int i=0;i<5;i++) { t=bc[(i+4)%5]^rotate(bc[(i+1)%5],1); st[i]^=t; st[i+5]^=t; st[i+10]^=t; st[i+15]^=t; st[i+20]^=t; }
        t=st[1];
        for(int i=0;i<24;i++) { int j=KECCAK_PI[i]; bc[0]=st[j]; st[j]=rotate(t,KECCAK_ROT[i]); t=bc[0]; }
        for(int i=0;i<25;i+=5) { bc[0]=st[i]; bc[1]=st[i+1]; bc[2]=st[i+2]; bc[3]=st[i+3]; bc[4]=st[i+4]; st[i]^=(~bc[1])&bc[2]; st[i+1]^=(~bc[2])&bc[3]; st[i+2]^=(~bc[3])&bc[4]; st[i+3]^=(~bc[4])&bc[0]; st[i+4]^=(~bc[0])&bc[1]; }
        st[0]^=KECCAK_RC[r];
    }
}

/* === SHA-256 for checksum === */
uint rotr(uint x, uint n) { return (x >> n) | (x << (32 - n)); }
uint ch(uint x, uint y, uint z) { return (x & y) ^ (~x & z); }
uint maj(uint x, uint y, uint z) { return (x & y) ^ (x & z) ^ (y & z); }
uint sigma0(uint x) { return rotr(x, 2) ^ rotr(x, 13) ^ rotr(x, 22); }
uint sigma1(uint x) { return rotr(x, 6) ^ rotr(x, 11) ^ rotr(x, 25); }
uint gamma0(uint x) { return rotr(x, 7) ^ rotr(x, 18) ^ (x >> 3); }
uint gamma1(uint x) { return rotr(x, 17) ^ rotr(x, 19) ^ (x >> 10); }

void sha256_block(uint *state, const uchar *data) {
    uint w[64];
    for(int i = 0; i < 16; i++) {
        w[i] = ((uint)data[i*4] << 24) | ((uint)data[i*4+1] << 16) | 
               ((uint)data[i*4+2] << 8) | (uint)data[i*4+3];
    }
    for(int i = 16; i < 64; i++) {
        w[i] = gamma1(w[i-2]) + w[i-7] + gamma0(w[i-15]) + w[i-16];
    }
    
    uint a = state[0], b = state[1], c = state[2], d = state[3];
    uint e = state[4], f = state[5], g = state[6], h = state[7];
    
    for(int i = 0; i < 64; i++) {
        uint t1 = h + sigma1(e) + ch(e, f, g) + SHA256_K[i] + w[i];
        uint t2 = sigma0(a) + maj(a, b, c);
        h = g; g = f; f = e; e = d + t1;
        d = c; c = b; b = a; a = t1 + t2;
    }
    
    state[0] += a; state[1] += b; state[2] += c; state[3] += d;
    state[4] += e; state[5] += f; state[6] += g; state[7] += h;
}

void sha256(const uchar *data, uint len, uchar *hash) {
    uint state[8] = {0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
                     0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19};
    
    uchar block[64];
    uint remaining = len;
    uint offset = 0;
    
    while(remaining >= 64) {
        for(int i = 0; i < 64; i++) block[i] = data[offset + i];
        sha256_block(state, block);
        offset += 64;
        remaining -= 64;
    }
    
    for(uint i = 0; i < remaining; i++) block[i] = data[offset + i];
    block[remaining] = 0x80;
    for(uint i = remaining + 1; i < 56; i++) block[i] = 0;
    
    if(remaining >= 56) {
        for(uint i = remaining + 1; i < 64; i++) block[i] = 0;
        sha256_block(state, block);
        for(int i = 0; i < 56; i++) block[i] = 0;
    }
    
    ulong bits = (ulong)len * 8;
    block[56] = (bits >> 56) & 0xff;
    block[57] = (bits >> 48) & 0xff;
    block[58] = (bits >> 40) & 0xff;
    block[59] = (bits >> 32) & 0xff;
    block[60] = (bits >> 24) & 0xff;
    block[61] = (bits >> 16) & 0xff;
    block[62] = (bits >> 8) & 0xff;
    block[63] = bits & 0xff;
    sha256_block(state, block);
    
    for(int i = 0; i < 8; i++) {
        hash[i*4] = (state[i] >> 24) & 0xff;
        hash[i*4+1] = (state[i] >> 16) & 0xff;
        hash[i*4+2] = (state[i] >> 8) & 0xff;
        hash[i*4+3] = state[i] & 0xff;
    }
}

/* === Base58 encoding for Tron === 
 * Tron uses 25 bytes: 0x41 + 20 address bytes + 4 checksum bytes
 * Output: up to 34 characters
 */
int base58_encode_25(const uchar *input, uchar *output) {
    // Convert 25 bytes to base58
    // Using big number division approach
    uchar temp[25];
    for(int i = 0; i < 25; i++) temp[i] = input[i];
    
    int out_idx = 0;
    uchar result[35];  // Max 34 chars + 1
    
    while(1) {
        // Check if all zeros
        int all_zero = 1;
        for(int i = 0; i < 25; i++) {
            if(temp[i] != 0) { all_zero = 0; break; }
        }
        if(all_zero) break;
        
        // Divide by 58
        uint remainder = 0;
        for(int i = 0; i < 25; i++) {
            uint val = remainder * 256 + temp[i];
            temp[i] = val / 58;
            remainder = val % 58;
        }
        result[out_idx++] = remainder;
    }
    
    // Count leading zeros in input
    int leading_zeros = 0;
    for(int i = 0; i < 25 && input[i] == 0; i++) leading_zeros++;
    
    // Reverse and add leading '1's
    int final_len = 0;
    for(int i = 0; i < leading_zeros; i++) {
        output[final_len++] = '1';
    }
    for(int i = out_idx - 1; i >= 0; i--) {
        output[final_len++] = BASE58_ALPHABET[result[i]];
    }
    
    return final_len;
}

/* === Main Kernel === */
__kernel void tron_generate_address(
    __global const uchar *base_point,      // BasePoint (Jacobian, 96 bytes)
    __global const uchar *table,           // Precomputed table (64 MB)
    __global uchar *output,                // Output: found_flag(4) + gid(4) + address(34) = 42 bytes
    __global uint *found_flag,             // Atomic flag
    __constant uchar *prefix,              // Prefix pattern
    __constant uchar *suffix,              // Suffix pattern
    __constant uchar *contains,            // Contains pattern
    uint prefix_len,
    uint suffix_len,
    uint contains_len
) {
    uint gid = get_global_id(0);
    uint lid = get_local_id(0);
    
    // Early exit if already found
    if(*found_flag != 0) return;
    
    // Shared memory for batch inversion
    __local uint256 Z_arr[WORKGROUP_SIZE];
    __local uint256 prefix_prod[WORKGROUP_SIZE];
    __local uint256 suffix_prod[WORKGROUP_SIZE];
    
    // 1. Load BasePoint
    point_j base;
    for(int i=0; i<8; i++) {
        base.x.w[i] = (uint)base_point[i*4] | ((uint)base_point[i*4+1]<<8) | 
                      ((uint)base_point[i*4+2]<<16) | ((uint)base_point[i*4+3]<<24);
        base.y.w[i] = (uint)base_point[32+i*4] | ((uint)base_point[32+i*4+1]<<8) | 
                      ((uint)base_point[32+i*4+2]<<16) | ((uint)base_point[32+i*4+3]<<24);
        base.z.w[i] = (uint)base_point[64+i*4] | ((uint)base_point[64+i*4+1]<<8) | 
                      ((uint)base_point[64+i*4+2]<<16) | ((uint)base_point[64+i*4+3]<<24);
    }
    
    // 2. Load Table point
    __global const uchar *te = table + (gid * 64);
    point_a tbl;
    for(int i=0; i<8; i++) {
        tbl.x.w[i] = (uint)te[i*4] | ((uint)te[i*4+1]<<8) | ((uint)te[i*4+2]<<16) | ((uint)te[i*4+3]<<24);
        tbl.y.w[i] = (uint)te[32+i*4] | ((uint)te[32+i*4+1]<<8) | ((uint)te[32+i*4+2]<<16) | ((uint)te[32+i*4+3]<<24);
    }
    
    // 3. Point addition
    point_j result;
    j_add_mixed(&result, &base, &tbl);
    uint256 resX = result.x, resY = result.y;
    
    // 4. Store Z for batch inversion
    uint256_copy_local(&Z_arr[lid], &result.z);
    uint256_copy_local(&prefix_prod[lid], &result.z);
    barrier(CLK_LOCAL_MEM_FENCE);
    
    // Parallel prefix product (up-sweep)
    for (uint stride = 1; stride < WORKGROUP_SIZE; stride <<= 1) {
        uint idx = (lid + 1) * (stride << 1) - 1;
        if (idx < WORKGROUP_SIZE) {
            uint256 left, right, prod;
            uint256_copy_from_local(&left, &prefix_prod[idx - stride]);
            uint256_copy_from_local(&right, &prefix_prod[idx]);
            mod_mul(&prod, &left, &right);
            uint256_copy_local(&prefix_prod[idx], &prod);
        }
        barrier(CLK_LOCAL_MEM_FENCE);
    }
    
    // Single inversion
    __local uint256 total_inv;
    if (lid == 0) {
        uint256 total;
        uint256_copy_from_local(&total, &prefix_prod[WORKGROUP_SIZE - 1]);
        uint256 inv;
        mod_inv(&inv, &total);
        uint256_copy_local(&total_inv, &inv);
        load_const(&prefix_prod[WORKGROUP_SIZE - 1], &ONE);
    }
    barrier(CLK_LOCAL_MEM_FENCE);
    
    // Down-sweep
    for (uint stride = WORKGROUP_SIZE >> 1; stride >= 1; stride >>= 1) {
        uint idx = (lid + 1) * (stride << 1) - 1;
        if (idx < WORKGROUP_SIZE) {
            uint left_idx = idx - stride;
            uint256 left_val, right_val, new_left, new_right;
            uint256_copy_from_local(&left_val, &prefix_prod[left_idx]);
            uint256_copy_from_local(&right_val, &prefix_prod[idx]);
            new_left = right_val;
            mod_mul(&new_right, &left_val, &right_val);
            uint256_copy_local(&prefix_prod[left_idx], &new_left);
            uint256_copy_local(&prefix_prod[idx], &new_right);
        }
        barrier(CLK_LOCAL_MEM_FENCE);
    }
    
    // Compute suffix products
    uint256_copy_local(&suffix_prod[lid], &Z_arr[lid]);
    barrier(CLK_LOCAL_MEM_FENCE);
    
    for (uint stride = 1; stride < WORKGROUP_SIZE; stride <<= 1) {
        uint256 val;
        if (lid + stride < WORKGROUP_SIZE) {
            uint256 mine, neighbor, prod;
            uint256_copy_from_local(&mine, &suffix_prod[lid]);
            uint256_copy_from_local(&neighbor, &suffix_prod[lid + stride]);
            mod_mul(&prod, &mine, &neighbor);
            val = prod;
        } else {
            uint256_copy_from_local(&val, &suffix_prod[lid]);
        }
        barrier(CLK_LOCAL_MEM_FENCE);
        uint256_copy_local(&suffix_prod[lid], &val);
        barrier(CLK_LOCAL_MEM_FENCE);
    }
    barrier(CLK_LOCAL_MEM_FENCE);
    
    // Compute invZ
    uint256 invZ, pref_val, tinv;
    uint256_copy_from_local(&pref_val, &prefix_prod[lid]);
    uint256_copy_from_local(&tinv, &total_inv);
    mod_mul(&invZ, &pref_val, &tinv);
    if (lid < WORKGROUP_SIZE - 1) {
        uint256 suff_val;
        uint256_copy_from_local(&suff_val, &suffix_prod[lid + 1]);
        mod_mul(&invZ, &invZ, &suff_val);
    }
    
    // 5. Compute affine coordinates
    uint256 z2, z3, affX, affY;
    mod_mul(&z2, &invZ, &invZ);
    mod_mul(&z3, &z2, &invZ);
    mod_mul(&affX, &resX, &z2);
    mod_mul(&affY, &resY, &z3);
    
    // 6. Serialize public key and compute Keccak
    ulong state[25] = {0};
    uchar pub[64];
    for(int i=0; i<8; i++) {
        uint wx = affX.w[7-i], wy = affY.w[7-i];
        pub[i*4]=(wx>>24); pub[i*4+1]=(wx>>16); pub[i*4+2]=(wx>>8); pub[i*4+3]=wx;
        pub[32+i*4]=(wy>>24); pub[32+i*4+1]=(wy>>16); pub[32+i*4+2]=(wy>>8); pub[32+i*4+3]=wy;
    }
    
    for(int i=0; i<8; i++) state[i] = ((ulong*)pub)[i];
    state[8] ^= 0x01;
    state[16] ^= 0x8000000000000000UL;
    keccak_f1600(state);
    
    // 7. Build Tron address data: 0x41 + last 20 bytes of Keccak
    uchar *h = (uchar*)state;
    uchar addr_data[25];
    addr_data[0] = 0x41;  // Tron mainnet prefix
    for(int i = 0; i < 20; i++) {
        addr_data[1 + i] = h[12 + i];
    }
    
    // 8. Double SHA256 for checksum
    uchar hash1[32], hash2[32];
    sha256(addr_data, 21, hash1);
    sha256(hash1, 32, hash2);
    
    // Append first 4 bytes of checksum
    addr_data[21] = hash2[0];
    addr_data[22] = hash2[1];
    addr_data[23] = hash2[2];
    addr_data[24] = hash2[3];
    
    // 9. Base58 encode
    uchar address[35];
    int addr_len = base58_encode_25(addr_data, address);
    
    // 10. Pattern matching
    bool match = true;
    
    // Check prefix (skip first char 'T' since all Tron addresses start with T)
    for(uint i = 0; i < prefix_len && match; i++) {
        if(address[1 + i] != prefix[i]) match = false;
    }
    
    // Check suffix
    for(uint i = 0; i < suffix_len && match; i++) {
        if(address[addr_len - suffix_len + i] != suffix[i]) match = false;
    }
    
    // Check contains - search for pattern in middle section
    if(match && contains_len > 0) {
        uint start_pos = 1 + prefix_len;  // Skip 'T' + prefix
        uint end_pos = addr_len - suffix_len;
        
        if(end_pos <= start_pos || contains_len > end_pos - start_pos) {
            match = false;
        } else {
            bool contains_found = false;
            
            // Sliding window search through middle section
            for(uint pos = start_pos; pos <= end_pos - contains_len && !contains_found; pos++) {
                bool pos_match = true;
                
                for(uint i = 0; i < contains_len && pos_match; i++) {
                    if(address[pos + i] != contains[i]) {
                        pos_match = false;
                    }
                }
                
                if(pos_match) {
                    contains_found = true;
                }
            }
            
            if(!contains_found) {
                match = false;
            }
        }
    }
    
    // 11. If match, write result
    if(match) {
        uint old = atomic_xchg(found_flag, 1);
        if(old == 0) {
            // Write GID
            output[0] = (gid >> 24) & 0xff;
            output[1] = (gid >> 16) & 0xff;
            output[2] = (gid >> 8) & 0xff;
            output[3] = gid & 0xff;
            // Write address
            for(int i = 0; i < addr_len && i < 34; i++) {
                output[4 + i] = address[i];
            }
            output[4 + addr_len] = 0; // null terminate
        }
    }
}
