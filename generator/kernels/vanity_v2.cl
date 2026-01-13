/* vanity_v2.cl - Phase 2 Fixed (Aliasing Safe)
 * - Robust math from Phase 1
 * - Aliasing-safe Jacobian functions (using local result buffers)
 */

#pragma OPENCL EXTENSION cl_khr_global_int32_base_atomics : enable

typedef struct { uint w[8]; } uint256;
// Jacobian Point (X, Y, Z) where x = X/Z^2, y = Y/Z^3
typedef struct { uint256 x; uint256 y; uint256 z; } point_j;
typedef struct { uint256 x; uint256 y; } point_a; // Affine

/* Constants */
__constant uint256 P_CONST = {{ 0xFFFFFC2F, 0xFFFFFFFE, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF }};
__constant uint256 GX = {{ 0x16F81798, 0x59F2815B, 0x2DCE28D9, 0x029BFCDB, 0xCE870B07, 0x55A06295, 0xF9DCBBAC, 0x79BE667E }};
__constant uint256 GY = {{ 0xFB10D4B8, 0x9C47D08F, 0xA6855419, 0xFD17B448, 0x0E1108A8, 0x5DA4FBFC, 0x26A3C465, 0x483ADA77 }};

/* Keccak Tables */
__constant ulong KECCAK_RC[24] = { 0x0000000000000001UL, 0x0000000000008082UL, 0x800000000000808aUL, 0x8000000080008000UL, 0x000000000000808bUL, 0x0000000080000001UL, 0x8000000080008081UL, 0x8000000000008009UL, 0x000000000000008aUL, 0x0000000000000088UL, 0x0000000080008009UL, 0x000000008000000aUL, 0x000000008000808bUL, 0x800000000000008bUL, 0x8000000000008089UL, 0x8000000000008003UL, 0x8000000000008002UL, 0x8000000000000080UL, 0x000000000000800aUL, 0x800000008000000aUL, 0x8000000080008081UL, 0x8000000000008080UL, 0x0000000080000001UL, 0x8000000080008008UL };
__constant int KECCAK_ROT[24] = { 1, 3, 6, 10, 15, 21, 28, 36, 45, 55, 2, 14, 27, 41, 56, 8, 25, 43, 62, 18, 39, 61, 20, 44 };
__constant int KECCAK_PI[24] = { 10, 7, 11, 17, 18, 3, 5, 16, 8, 21, 24, 4, 15, 23, 19, 13, 12, 2, 20, 14, 22, 9, 6, 1 };

/* === Low Level Math (Robust) === */
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

/* Modular Math (Robust) */
void mod_add(uint256 *r, const uint256 *a, const uint256 *b) {
    uint256 P; load_const(&P, &P_CONST);
    if(add_c(r,a,b) || gte(r,&P)) sub_b(r,r,&P);
}
void mod_sub(uint256 *r, const uint256 *a, const uint256 *b) {
    uint256 P; load_const(&P, &P_CONST);
    if(sub_b(r,a,b)) add_c(r,r,&P);
}

/* ROBUST MOD MUL from Phase 1 Fix 3 */
void mod_mul(uint256 *result, const uint256 *a, const uint256 *b) {
    ulong u[16] = {0};
    
    // 1. Schoolbook multiplication
    for (int i = 0; i < 8; i++) {
        ulong carry = 0;
        for (int j = 0; j < 8; j++) {
            ulong prod = (ulong)a->w[i] * (ulong)b->w[j] + u[i+j] + carry;
            u[i+j] = prod & 0xFFFFFFFF;
            carry = prod >> 32;
        }
        u[i+8] = carry;
    }

    // 2. Reduction: Reduce high words (u[8]..u[15])
    for (int iter = 0; iter < 6; iter++) {
        int high_zero = 1;
        for(int k=8; k<16; k++) if(u[k] != 0) high_zero = 0;
        if(high_zero) break;

        uint high[8];
        for(int k=0; k<8; k++) {
            high[k] = (uint)u[k+8];
            u[k+8] = 0;
        }

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
    while (gte(&final_res, &P)) {
        sub_b(&final_res, &final_res, &P);
    }
    
    *result = final_res;
}

/* ROBUST MOD POW / INV */
void mod_pow(uint256 *result, const uint256 *base, const uint256 *exp) {
    uint256 r; set_u64(&r, 1);
    uint256 b; uint256_copy(&b, base);
    
    for (int i = 0; i < 8; i++) {
        uint w = exp->w[i];
        for (int j = 0; j < 32; j++) {
            if ((w >> j) & 1) {
                mod_mul(&r, &r, &b);
            }
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

/* === Jacobian ECC (Aliasing Safe) === */
void j_double(point_j *r, const point_j *p) {
    if(is_zero(&p->z)) { *r=*p; return; }
    
    // Locals for intermediate and final results to avoid aliasing (r == p)
    uint256 rx, ry, rz;
    uint256 A, B, C, D, E, F, tmp;
    
    mod_mul(&A, &p->x, &p->x);
    mod_mul(&B, &p->y, &p->y);
    mod_mul(&C, &B, &B);
    
    mod_add(&tmp, &p->x, &B);
    mod_mul(&D, &tmp, &tmp);
    mod_sub(&D, &D, &A);
    mod_sub(&D, &D, &C);
    mod_add(&D, &D, &D); // D = 2 * (...)
    
    mod_add(&E, &A, &A);
    mod_add(&E, &E, &A); // E = 3 * A
    
    mod_mul(&F, &E, &E);
    
    // Compute rx
    mod_sub(&rx, &F, &D);
    mod_sub(&rx, &rx, &D); // X3
    
    // Compute ry
    mod_sub(&tmp, &D, &rx);
    mod_mul(&ry, &E, &tmp);
    
    mod_add(&tmp, &C, &C);
    mod_add(&tmp, &tmp, &tmp);
    mod_add(&tmp, &tmp, &tmp); // 8 * C
    mod_sub(&ry, &ry, &tmp); // Y3
    
    // Compute rz (uses p->y and p->z - safe now as ry is local)
    mod_mul(&rz, &p->y, &p->z);
    mod_add(&rz, &rz, &rz); // Z3
    
    // Copy back
    r->x = rx; r->y = ry; r->z = rz;
}

void j_add(point_j *r, const point_j *p, const point_j *q) {
    if(is_zero(&p->z)) { *r=*q; return; }
    if(is_zero(&q->z)) { *r=*p; return; }
    
    uint256 rx, ry, rz;
    uint256 Z1Z1, Z2Z2, U1, U2, S1, S2, H, I, J, r_val, V, tmp;
    
    mod_mul(&Z1Z1, &p->z, &p->z);
    mod_mul(&Z2Z2, &q->z, &q->z);
    
    mod_mul(&U1, &p->x, &Z2Z2);
    mod_mul(&U2, &q->x, &Z1Z1);
    
    mod_mul(&S1, &p->y, &q->z); mod_mul(&S1, &S1, &Z2Z2);
    mod_mul(&S2, &q->y, &p->z); mod_mul(&S2, &S2, &Z1Z1);
    
    if(!gte(&U1,&U2) && !gte(&U2,&U1)) { // U1 == U2
        if(!gte(&S1,&S2) && !gte(&S2,&S1)) j_double(r, p);
        else { set_u64(&r->x,0); set_u64(&r->y,0); set_u64(&r->z,0); }
        return;
    }
    
    mod_sub(&H, &U2, &U1);
    mod_add(&I, &H, &H); mod_mul(&I, &I, &I); // I = (2H)^2
    mod_mul(&J, &H, &I); // J = H * I
    mod_sub(&r_val, &S2, &S1);
    mod_add(&r_val, &r_val, &r_val); // r = 2(S2-S1)
    
    mod_mul(&V, &U1, &I);
    
    // Compute rx
    mod_mul(&rx, &r_val, &r_val);
    mod_sub(&rx, &rx, &J);
    mod_sub(&rx, &rx, &V);
    mod_sub(&rx, &rx, &V); // X3
    
    // Compute ry
    mod_sub(&ry, &V, &rx);
    mod_mul(&ry, &ry, &r_val);
    mod_mul(&tmp, &S1, &J);
    mod_add(&tmp, &tmp, &tmp);
    mod_sub(&ry, &ry, &tmp); // Y3
    
    // Compute rz
    mod_add(&rz, &p->z, &q->z);
    mod_mul(&rz, &rz, &rz);
    mod_sub(&rz, &rz, &Z1Z1);
    mod_sub(&rz, &rz, &Z2Z2);
    mod_mul(&rz, &rz, &H); // Z3
    
    // Copy back
    r->x = rx; r->y = ry; r->z = rz;
}

void scalar_mult_j(point_a *res, const uint256 *k) {
    point_j r, g;
    // Load G into Jacobian (Z=1)
    load_const(&g.x, &GX); load_const(&g.y, &GY); set_u64(&g.z, 1);
    set_u64(&r.x, 0); set_u64(&r.y, 0); set_u64(&r.z, 0); // Infinity
    
    for(int i=7; i>=0; i--) {
        for(int j=31; j>=0; j--) {
            j_double(&r, &r);
            if((k->w[i] >> j) & 1) j_add(&r, &r, &g);
        }
    }
    
    // Convert back to Affine: x = X/Z^2, y = Y/Z^3
    if(is_zero(&r.z)) { set_u64(&res->x,0); set_u64(&res->y,0); return; }
    
    uint256 z2, z3, invZ;
    mod_inv(&invZ, &r.z);      // ONLY ONE DIVISION AT THE END!
    mod_mul(&z2, &invZ, &invZ);
    mod_mul(&z3, &z2, &invZ);
    
    mod_mul(&res->x, &r.x, &z2);
    mod_mul(&res->y, &r.y, &z3);
}

/* === Main Kernel === */
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

__kernel void compute_address(
    __global const uchar *base_key, // Base private key
    __global uchar *output_result,  // Only write if found (optional) or debug
    __global uint *found_flag       // Simple flag to signal found
) {
    ulong gid = get_global_id(0);
    
    // 1. Calculate Private Key = Base + GID
    uint256 k;
    for(int i=0; i<8; i++) k.w[i] = ((uint)base_key[(7-i)*4]<<24)|((uint)base_key[(7-i)*4+1]<<16)|((uint)base_key[(7-i)*4+2]<<8)|((uint)base_key[(7-i)*4+3]);
    
    // Add GID to low words (handle carry)
    ulong carry = gid;
    for(int i=0; i<8 && carry; i++) {
        ulong sum = (ulong)k.w[i] + carry;
        k.w[i] = (uint)sum;
        carry = sum >> 32;
    }
    
    // 2. Compute Public Key (using Jacobian - Fast!)
    point_a P;
    scalar_mult_j(&P, &k);
    
    // 3. Hash
    ulong state[25]={0};
    uchar pub[64];
    for(int i=0;i<8;i++) {
        uint wx=P.x.w[7-i]; uint wy=P.y.w[7-i];
        pub[i*4]=(wx>>24); pub[i*4+1]=(wx>>16); pub[i*4+2]=(wx>>8); pub[i*4+3]=wx;
        pub[32+i*4]=(wy>>24); pub[32+i*4+1]=(wy>>16); pub[32+i*4+2]=(wy>>8); pub[32+i*4+3]=wy;
    }
    for(int i=0;i<8;i++) state[i]=((ulong*)pub)[i];
    state[8]^=0x01; state[16]^=0x8000000000000000UL;
    keccak_f1600(state);
    
    // 4. Verification (Batch Mode) - Write all results
    // In production, we would check "prefix" here and only write if found.
    // For now, we want to verify ALL threads are working correctly.
    uchar *h=(uchar*)state;
    for(int i=0;i<20;i++) output_result[gid*20 + i] = h[12+i];
}