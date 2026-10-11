
experiments/direct-simd-map/results/native-main/map-true.bin:     file format binary


Disassembly of section .data:

0000000000000000 <.data>:
   0:	48 89 f3             	mov    %rsi,%rbx
   3:	4c 8b bb e0 fe ff ff 	mov    -0x120(%rbx),%r15
   a:	51                   	push   %rcx
   b:	48 8b 07             	mov    (%rdi),%rax
   e:	48 8b 4f 08          	mov    0x8(%rdi),%rcx
  12:	48 8b 57 10          	mov    0x10(%rdi),%rdx
  16:	4c 8b 47 18          	mov    0x18(%rdi),%r8
  1a:	f3 0f 10 47 20       	movss  0x20(%rdi),%xmm0
  1f:	f3 0f 10 4f 28       	movss  0x28(%rdi),%xmm1
  24:	e8 07 00 00 00       	call   0x30
  29:	59                   	pop    %rcx
  2a:	48 89 01             	mov    %rax,(%rcx)
  2d:	c3                   	ret
  2e:	66 90                	xchg   %ax,%ax
  30:	48 81 ec 38 00 00 00 	sub    $0x38,%rsp
  37:	41 89 c4             	mov    %eax,%r12d
  3a:	41 89 cd             	mov    %ecx,%r13d
  3d:	41 89 d6             	mov    %edx,%r14d
  40:	45 89 c3             	mov    %r8d,%r11d
  43:	f3 44 0f 10 e8       	movss  %xmm0,%xmm13
  48:	f3 44 0f 10 f1       	movss  %xmm1,%xmm14
  4d:	31 c0                	xor    %eax,%eax
  4f:	66 45 0f 57 e4       	xorpd  %xmm12,%xmm12
  54:	45 85 db             	test   %r11d,%r11d
  57:	0f 84 5f 00 00 00    	je     0xbc
  5d:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  62:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
  69:	00 00 
  6b:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  70:	45 89 ed             	mov    %r13d,%r13d
  73:	49 8d 7d 04          	lea    0x4(%r13),%rdi
  77:	4c 39 ff             	cmp    %r15,%rdi
  7a:	0f 87 64 00 00 00    	ja     0xe4
  80:	c4 a1 12 59 04 2b    	vmulss (%rbx,%r13,1),%xmm13,%xmm0
  86:	c4 41 7a 58 e6       	vaddss %xmm14,%xmm0,%xmm12
  8b:	45 89 e4             	mov    %r12d,%r12d
  8e:	49 8d 7c 24 04       	lea    0x4(%r12),%rdi
  93:	4c 39 ff             	cmp    %r15,%rdi
  96:	0f 87 48 00 00 00    	ja     0xe4
  9c:	f3 46 0f 11 24 23    	movss  %xmm12,(%rbx,%r12,1)
  a2:	41 83 c4 04          	add    $0x4,%r12d
  a6:	41 83 c5 04          	add    $0x4,%r13d
  aa:	41 83 c6 01          	add    $0x1,%r14d
  ae:	45 39 de             	cmp    %r11d,%r14d
  b1:	0f 85 b9 ff ff ff    	jne    0x70
  b7:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  bc:	f3 41 0f 10 c4       	movss  %xmm12,%xmm0
  c1:	66 0f 7e c7          	movd   %xmm0,%edi
  c5:	44 89 e6             	mov    %r12d,%esi
  c8:	44 31 ee             	xor    %r13d,%esi
  cb:	44 89 f5             	mov    %r14d,%ebp
  ce:	31 f5                	xor    %esi,%ebp
  d0:	31 ef                	xor    %ebp,%edi
  d2:	48 89 7c 24 28       	mov    %rdi,0x28(%rsp)
  d7:	48 8b 44 24 28       	mov    0x28(%rsp),%rax
  dc:	48 81 c4 38 00 00 00 	add    $0x38,%rsp
  e3:	c3                   	ret
  e4:	b8 ff ff ff ff       	mov    $0xffffffff,%eax
  e9:	48 8b 73 98          	mov    -0x68(%rbx),%rsi
  ed:	c7 46 10 01 00 00 00 	movl   $0x1,0x10(%rsi)
  f4:	89 46 14             	mov    %eax,0x14(%rsi)
  f7:	c7 06 03 00 00 00    	movl   $0x3,(%rsi)
  fd:	48 8b 63 e8          	mov    -0x18(%rbx),%rsp
 101:	c3                   	ret
