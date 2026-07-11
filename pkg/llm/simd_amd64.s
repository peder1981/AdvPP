#include "textflag.h"

// mask3 = 0x03 repetido em todo byte (32 vezes).
DATA mask3<>+0(SB)/8, $0x0303030303030303
DATA mask3<>+8(SB)/8, $0x0303030303030303
DATA mask3<>+16(SB)/8, $0x0303030303030303
DATA mask3<>+24(SB)/8, $0x0303030303030303
GLOBL mask3<>(SB), RODATA, $32

// ones16 = 1 repetido em toda lane de 16 bits (16 vezes), usado para somar
// pares adjacentes de int16 em int32 via VPMADDWD.
DATA ones16<>+0(SB)/8, $0x0001000100010001
DATA ones16<>+8(SB)/8, $0x0001000100010001
DATA ones16<>+16(SB)/8, $0x0001000100010001
DATA ones16<>+24(SB)/8, $0x0001000100010001
GLOBL ones16<>(SB), RODATA, $32

// func dotI2SBlocksAVX2(packed []byte, q []int8, nBlocks int) int32
TEXT ·dotI2SBlocksAVX2(SB), NOSPLIT, $0-60
	MOVQ packed_base+0(FP), SI
	MOVQ q_base+24(FP), DI
	MOVQ nBlocks+48(FP), CX

	VMOVDQU mask3<>(SB), Y1
	VMOVDQU ones16<>(SB), Y11
	VPXOR   Y10, Y10, Y10 // acumulador de 8x int32 = 0

	TESTQ CX, CX
	JEQ   done

loop:
	VMOVDQU (SI), Y0 // 32 bytes empacotados = 128 valores ternários

	// extrai os 4 códigos de 2 bits por byte (bits 7:6, 5:4, 3:2, 1:0),
	// igual a dequantize_row_i2_s em ggml-quants.c.
	VPSRLW $6, Y0, Y2
	VPAND  Y1, Y2, Y2 // codes0 = (raw>>6)&3  -> valores em q[0:32]
	VPSRLW $4, Y0, Y3
	VPAND  Y1, Y3, Y3 // codes1 = (raw>>4)&3  -> valores em q[32:64]
	VPSRLW $2, Y0, Y4
	VPAND  Y1, Y4, Y4 // codes2 = (raw>>2)&3  -> valores em q[64:96]
	VPAND  Y1, Y0, Y5 // codes3 = raw&3       -> valores em q[96:128]

	VMOVDQU (DI), Y6    // q[0:32]
	VMOVDQU 32(DI), Y7  // q[32:64]
	VMOVDQU 64(DI), Y8  // q[64:96]
	VMOVDQU 96(DI), Y9  // q[96:128]

	// VPMADDUBSW trata o primeiro operando lógico (aqui: codes, listado
	// como vvvv/2º operando em asm Go) como unsigned e o segundo (q,
	// listado como rm/1º operando) como signed — exatamente a mesma
	// convenção usada por ggml (código bruto 0..2 vezes ativação int8).
	VPMADDUBSW Y6, Y2, Y2
	VPMADDUBSW Y7, Y3, Y3
	VPMADDUBSW Y8, Y4, Y4
	VPMADDUBSW Y9, Y5, Y5

	VPADDW Y3, Y2, Y2
	VPADDW Y5, Y4, Y4
	VPADDW Y4, Y2, Y2 // soma dos 4 sub-blocos, 16x int16 (faixa segura por bloco)

	VPMADDWD Y11, Y2, Y2 // widen para 8x int32
	VPADDD   Y2, Y10, Y10

	ADDQ $32, SI
	ADDQ $128, DI
	DECQ CX
	JNZ  loop

done:
	// soma horizontal de Y10 (8x int32) em um escalar. VZEROUPPER só
	// depois — ele zera os 128 bits altos de todo YMM, e é exatamente de
	// lá que VEXTRACTI128 precisa ler primeiro.
	VEXTRACTI128 $1, Y10, X0
	VPADDD       X0, X10, X10  // X10 = [a+e, b+f, c+g, d+h]
	VPSHUFD      $0xEE, X10, X0
	VPADDD       X0, X10, X10  // X10[0] = (a+e)+(c+g), X10[1] = (b+f)+(d+h)
	VPSHUFD      $0x55, X10, X0
	VPADDD       X0, X10, X10  // X10[0] = soma final
	VMOVD        X10, AX
	MOVL         AX, ret+56(FP)
	VZEROUPPER
	RET

// func dotF16BlocksAVX2(rowF16 []byte, x []float32, nBlocks int) float32
//
// Processa nBlocks blocos de 8 valores F16 (16 bytes empacotados) cada:
// VCVTPH2PS converte os 8 halfs em 8 float32 direto em hardware (mesmo
// formato IEEE 754 half que Float16ToFloat32 decodifica em Go — sem
// diferença de arredondamento, é a MESMA conversão feita pela CPU), depois
// VFMADD231PS acumula convertido*x em Y10 (8 lanes de float32, uma
// soma parcial por lane; reduzidas a um escalar só no final, igual ao
// kernel inteiro acima).
TEXT ·dotF16BlocksAVX2(SB), NOSPLIT, $0-60
	MOVQ rowF16_base+0(FP), SI
	MOVQ x_base+24(FP), DI
	MOVQ nBlocks+48(FP), CX

	VPXOR Y10, Y10, Y10 // acumulador de 8x float32 = 0

	TESTQ CX, CX
	JEQ   doneF16

loopF16:
	VMOVDQU   (SI), X0    // 16 bytes = 8x half-float empacotados
	VCVTPH2PS X0, Y0      // Y0 = 8x float32 convertido em hardware
	VMOVUPS   (DI), Y1    // 8 floats de x
	VFMADD231PS Y1, Y0, Y10 // Y10 += Y0*Y1

	ADDQ $16, SI
	ADDQ $32, DI
	DECQ CX
	JNZ  loopF16

doneF16:
	// soma horizontal de Y10 (8x float32), mesmo padrão do kernel inteiro
	// acima (VPSHUFD só rearranja bytes de 32 bits, não interpreta como
	// int/float — reaproveitável aqui igual, só trocando PADDD por ADDPS).
	VEXTRACTF128 $1, Y10, X0
	VADDPS       X0, X10, X10 // X10 = [a+e, b+f, c+g, d+h]
	VPSHUFD      $0xEE, X10, X0
	VADDPS       X0, X10, X10 // X10[0] = (a+e)+(c+g), X10[1] = (b+f)+(d+h)
	VPSHUFD      $0x55, X10, X0
	VADDPS       X0, X10, X10 // X10[0] = soma final
	VMOVSS       X10, ret+56(FP)
	VZEROUPPER
	RET
