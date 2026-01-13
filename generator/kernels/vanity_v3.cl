/* vanity_v3.cl - Phase 3: Precomputed Tables Optimization
 * - Uses lookup table instead of scalar multiplication
 * - j_add_mixed: Jacobian + Affine → Jacobian (25% faster than J+J)
 * - Single point addition per thread instead of 256 double-and-add ops
 */

#pragma OPENCL EXTENSION cl_khr_global_int32_base_atomics : enable

typedef struct { uint w[8]; } uint256;
typedef struct { uint256 x; uint256 y; uint256 z; } point_j; // Jacobian
typedef struct { uint256 x; uint256 y; } point_a;             // Affine

/* Constants */
__constant uint256 P_CONST = {{ 0xFFFFFC2F, 0xFFFFFFFE, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF }};

/* Keccak Tables */
__constant ulong KECCAK_RC[24] = { 0x0000000000000001UL, 0x0000000000008082UL, 0x800000000000808aUL, 0x8000000080008000UL, 0x000000000000808bUL, 0x0000000080000001UL, 0x8000000080008081UL, 0x8000000000008009UL, 0x000000000000008aUL, 0x0000000000000088UL, 0x0000000080008009UL, 0x000000008000000aUL, 0x000000008000808bUL, 0x800000000000008bUL, 0x8000000000008089UL, 0x8000000000008003UL, 0x8000000000008002UL, 0x8000000000000080UL, 0x000000000000800aUL, 0x800000008000000aUL, 0x8000000080008081UL, 0x8000000000008080UL, 0x0000000080000001UL, 0x8000000080008008UL };
__constant int KECCAK_ROT[24] = { 1, 3, 6, 10, 15, 21, 28, 36, 45, 55, 2, 14, 27, 41, 56, 8, 25, 43, 62, 18, 39, 61, 20, 44 };
__constant int KECCAK_PI[24] = { 10, 7, 11, 17, 18, 3, 5, 16, 8, 21, 24, 4, 15, 23, 19, 13, 12, 2, 20, 14, 22, 9, 6, 1 };

/* === Low Level Math === */
void load_const(uint256 *d, __constant const uint256 *s) { for(int i=0;i<8;i++) d->w[i]=s->w[i]; }
int is_zero(const uint256 *a) { for(int i=0;i<8;i++) if(a->w[i]!=0) return 0; return 1; }
void set_u64(uint256 *a, ulong v) { a->w[0]=(uint)v; a->w[1]=(uint)(v>>32); for(int i=2;i<8;i++) a->w[i]=0; }
void uint256_copy(uint256 *d, const uint256 *s) { for(int i=0;i<8;i++) d->w[i]=s->w[i]; }

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

/* Modular Math */
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

/* === Mixed Point Addition: Jacobian + Affine → Jacobian ===
 * This is ~25% faster than Jacobian + Jacobian because Z2 = 1
 * Formula from: https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-0.html#addition-madd-2007-bl
 */
void j_add_mixed(point_j *r, const point_j *p, const point_a *q) {
    // If P is infinity, return Q (converted to Jacobian)
    if(is_zero(&p->z)) {
        uint256_copy(&r->x, &q->x);
        uint256_copy(&r->y, &q->y);
        set_u64(&r->z, 1);
        return;
    }
    
    // If Q is infinity (both coords zero), return P
    if(is_zero(&q->x) && is_zero(&q->y)) {
        *r = *p;
        return;
    }
    
    uint256 rx, ry, rz;
    uint256 Z1Z1, U2, S2, H, HH, I, J, r_val, V, tmp;
    
    // Z1Z1 = Z1^2
    mod_mul(&Z1Z1, &p->z, &p->z);
    
    // U2 = X2 * Z1Z1 (X2 is affine, so no Z2)
    mod_mul(&U2, &q->x, &Z1Z1);
    
    // S2 = Y2 * Z1 * Z1Z1
    mod_mul(&S2, &q->y, &p->z);
    mod_mul(&S2, &S2, &Z1Z1);
    
    // H = U2 - X1
    mod_sub(&H, &U2, &p->x);
    
    // HH = H^2
    mod_mul(&HH, &H, &H);
    
    // I = 4 * HH
    mod_add(&I, &HH, &HH);
    mod_add(&I, &I, &I);
    
    // J = H * I
    mod_mul(&J, &H, &I);
    
    // r = 2 * (S2 - Y1)
    mod_sub(&r_val, &S2, &p->y);
    mod_add(&r_val, &r_val, &r_val);
    
    // V = X1 * I
    mod_mul(&V, &p->x, &I);
    
    // X3 = r^2 - J - 2*V
    mod_mul(&rx, &r_val, &r_val);
    mod_sub(&rx, &rx, &J);
    mod_sub(&rx, &rx, &V);
    mod_sub(&rx, &rx, &V);
    
    // Y3 = r * (V - X3) - 2 * Y1 * J
    mod_sub(&tmp, &V, &rx);
    mod_mul(&ry, &r_val, &tmp);
    mod_mul(&tmp, &p->y, &J);
    mod_add(&tmp, &tmp, &tmp);
    mod_sub(&ry, &ry, &tmp);
    
    // Z3 = 2 * Z1 * H (simplified because Z2 = 1)
    mod_mul(&rz, &p->z, &H);
    mod_add(&rz, &rz, &rz);
    
    r->x = rx;
    r->y = ry;
    r->z = rz;
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

/* === Main Kernel ===
 * BasePoint: Jacobian point (base * G) computed on CPU - 96 bytes
 * Table: Precomputed affine points (i * G) - 64 bytes each
 * 
 * For thread i: PublicKey = BasePoint + Table[i] = (base + i) * G
 */
__kernel void compute_address(
    __global const uchar *base_point,    // Jacobian: 96 bytes (X:32, Y:32, Z:32)
    __global const uchar *table,         // Affine table: 64 bytes per point
    __global uchar *output,              // Output addresses: 20 bytes per thread
    __global uint *found_flag            // Flag for match found
) {
    uint gid = get_global_id(0);
    
    // 1. Load BasePoint (Jacobian) from host
    point_j base;
    for(int i=0; i<8; i++) {
        // Little-endian load
        base.x.w[i] = (uint)base_point[i*4] | ((uint)base_point[i*4+1]<<8) | 
                      ((uint)base_point[i*4+2]<<16) | ((uint)base_point[i*4+3]<<24);
        base.y.w[i] = (uint)base_point[32+i*4] | ((uint)base_point[32+i*4+1]<<8) | 
                      ((uint)base_point[32+i*4+2]<<16) | ((uint)base_point[32+i*4+3]<<24);
        base.z.w[i] = (uint)base_point[64+i*4] | ((uint)base_point[64+i*4+1]<<8) | 
                      ((uint)base_point[64+i*4+2]<<16) | ((uint)base_point[64+i*4+3]<<24);
    }
    
    // 2. Load Table[gid] (Affine) - each point is 64 bytes
    __global const uchar *table_entry = table + (gid * 64);
    point_a tbl_point;
    for(int i=0; i<8; i++) {
        tbl_point.x.w[i] = (uint)table_entry[i*4] | ((uint)table_entry[i*4+1]<<8) | 
                           ((uint)table_entry[i*4+2]<<16) | ((uint)table_entry[i*4+3]<<24);
        tbl_point.y.w[i] = (uint)table_entry[32+i*4] | ((uint)table_entry[32+i*4+1]<<8) | 
                           ((uint)table_entry[32+i*4+2]<<16) | ((uint)table_entry[32+i*4+3]<<24);
    }
    
    // 3. Compute PublicKey = BasePoint + Table[gid] (Single point addition!)
    point_j result;
    j_add_mixed(&result, &base, &tbl_point);
    
    // 4. Convert Jacobian to Affine: x = X/Z^2, y = Y/Z^3
    uint256 z2, z3, invZ;
    mod_inv(&invZ, &result.z);
    mod_mul(&z2, &invZ, &invZ);
    mod_mul(&z3, &z2, &invZ);
    
    uint256 affX, affY;
    mod_mul(&affX, &result.x, &z2);
    mod_mul(&affY, &result.y, &z3);
    
    // 5. Serialize public key (64 bytes, big-endian for Keccak)
    ulong state[25] = {0};
    uchar pub[64];
    for(int i=0; i<8; i++) {
        uint wx = affX.w[7-i];
        uint wy = affY.w[7-i];
        pub[i*4]   = (wx>>24);
        pub[i*4+1] = (wx>>16);
        pub[i*4+2] = (wx>>8);
        pub[i*4+3] = wx;
        pub[32+i*4]   = (wy>>24);
        pub[32+i*4+1] = (wy>>16);
        pub[32+i*4+2] = (wy>>8);
        pub[32+i*4+3] = wy;
    }
    
    // 6. Keccak-256 hash
    for(int i=0; i<8; i++) state[i] = ((ulong*)pub)[i];
    state[8] ^= 0x01;
    state[16] ^= 0x8000000000000000UL;
    keccak_f1600(state);
    
    // 7. Extract last 20 bytes as address
    uchar *h = (uchar*)state;
    for(int i=0; i<20; i++) {
        output[gid*20 + i] = h[12+i];
    }
}
