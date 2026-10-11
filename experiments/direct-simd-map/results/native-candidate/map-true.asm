
experiments/direct-simd-map/results/native-candidate/map-true.bin:     file format binary


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
  30:	48 81 ec 88 00 00 00 	sub    $0x88,%rsp
  37:	41 89 c4             	mov    %eax,%r12d
  3a:	41 89 cd             	mov    %ecx,%r13d
  3d:	41 89 d6             	mov    %edx,%r14d
  40:	45 89 c3             	mov    %r8d,%r11d
  43:	f3 44 0f 10 e8       	movss  %xmm0,%xmm13
  48:	f3 44 0f 10 f1       	movss  %xmm1,%xmm14
  4d:	31 c0                	xor    %eax,%eax
  4f:	66 45 0f 57 e4       	xorpd  %xmm12,%xmm12
  54:	45 85 db             	test   %r11d,%r11d
  57:	0f 84 55 02 00 00    	je     0x2b2
  5d:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  62:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
  69:	00 00 
  6b:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  70:	e9 5c 00 00 00       	jmp    0xd1
  75:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
  7c:	00 00 
  7e:	66 90                	xchg   %ax,%ax
  80:	45 89 ed             	mov    %r13d,%r13d
  83:	49 8d 7d 04          	lea    0x4(%r13),%rdi
  87:	4c 39 ff             	cmp    %r15,%rdi
  8a:	0f 87 4a 02 00 00    	ja     0x2da
  90:	c4 a1 12 59 04 2b    	vmulss (%rbx,%r13,1),%xmm13,%xmm0
  96:	c4 41 7a 58 e6       	vaddss %xmm14,%xmm0,%xmm12
  9b:	45 89 e4             	mov    %r12d,%r12d
  9e:	49 8d 7c 24 04       	lea    0x4(%r12),%rdi
  a3:	4c 39 ff             	cmp    %r15,%rdi
  a6:	0f 87 2e 02 00 00    	ja     0x2da
  ac:	f3 46 0f 11 24 23    	movss  %xmm12,(%rbx,%r12,1)
  b2:	41 83 c4 04          	add    $0x4,%r12d
  b6:	41 83 c5 04          	add    $0x4,%r13d
  ba:	41 83 c6 01          	add    $0x1,%r14d
  be:	45 39 de             	cmp    %r11d,%r14d
  c1:	0f 85 b9 ff ff ff    	jne    0x80
  c7:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  cc:	e9 e1 01 00 00       	jmp    0x2b2
  d1:	44 89 df             	mov    %r11d,%edi
  d4:	44 89 f6             	mov    %r14d,%esi
  d7:	39 f7                	cmp    %esi,%edi
  d9:	0f 86 a1 ff ff ff    	jbe    0x80
  df:	29 f7                	sub    %esi,%edi
  e1:	83 ff 10             	cmp    $0x10,%edi
  e4:	0f 82 96 ff ff ff    	jb     0x80
  ea:	89 7c 24 68          	mov    %edi,0x68(%rsp)
  ee:	44 89 e7             	mov    %r12d,%edi
  f1:	48 89 7c 24 28       	mov    %rdi,0x28(%rsp)
  f6:	44 89 ef             	mov    %r13d,%edi
  f9:	48 89 7c 24 30       	mov    %rdi,0x30(%rsp)
  fe:	66 4c 0f 7e ef       	movq   %xmm13,%rdi
 103:	48 89 7c 24 38       	mov    %rdi,0x38(%rsp)
 108:	66 4c 0f 7e f7       	movq   %xmm14,%rdi
 10d:	48 89 7c 24 40       	mov    %rdi,0x40(%rsp)
 112:	66 4c 0f 7e e7       	movq   %xmm12,%rdi
 117:	48 89 7c 24 48       	mov    %rdi,0x48(%rsp)
 11c:	44 89 f7             	mov    %r14d,%edi
 11f:	48 89 7c 24 50       	mov    %rdi,0x50(%rsp)
 124:	44 89 df             	mov    %r11d,%edi
 127:	48 89 7c 24 58       	mov    %rdi,0x58(%rsp)
 12c:	bf 00 00 00 00       	mov    $0x0,%edi
 131:	44 8b 4c 24 30       	mov    0x30(%rsp),%r9d
 136:	44 01 cf             	add    %r9d,%edi
 139:	8b 74 24 68          	mov    0x68(%rsp),%esi
 13d:	48 83 ee 01          	sub    $0x1,%rsi
 141:	48 6b f6 04          	imul   $0x4,%rsi,%rsi
 145:	48 83 c6 04          	add    $0x4,%rsi
 149:	48 89 fd             	mov    %rdi,%rbp
 14c:	48 01 f5             	add    %rsi,%rbp
 14f:	49 b9 00 00 00 00 01 	movabs $0x100000000,%r9
 156:	00 00 00 
 159:	4c 39 cd             	cmp    %r9,%rbp
 15c:	0f 87 1e ff ff ff    	ja     0x80
 162:	48 3b ab e0 fe ff ff 	cmp    -0x120(%rbx),%rbp
 169:	0f 87 11 ff ff ff    	ja     0x80
 16f:	48 89 7c 24 70       	mov    %rdi,0x70(%rsp)
 174:	bf 00 00 00 00       	mov    $0x0,%edi
 179:	44 8b 4c 24 28       	mov    0x28(%rsp),%r9d
 17e:	44 01 cf             	add    %r9d,%edi
 181:	8b 74 24 68          	mov    0x68(%rsp),%esi
 185:	48 83 ee 01          	sub    $0x1,%rsi
 189:	48 6b f6 04          	imul   $0x4,%rsi,%rsi
 18d:	48 83 c6 04          	add    $0x4,%rsi
 191:	48 89 fd             	mov    %rdi,%rbp
 194:	48 01 f5             	add    %rsi,%rbp
 197:	49 b9 00 00 00 00 01 	movabs $0x100000000,%r9
 19e:	00 00 00 
 1a1:	4c 39 cd             	cmp    %r9,%rbp
 1a4:	0f 87 d6 fe ff ff    	ja     0x80
 1aa:	48 3b ab e0 fe ff ff 	cmp    -0x120(%rbx),%rbp
 1b1:	0f 87 c9 fe ff ff    	ja     0x80
 1b7:	48 89 7c 24 78       	mov    %rdi,0x78(%rsp)
 1bc:	48 8b 7c 24 78       	mov    0x78(%rsp),%rdi
 1c1:	48 8b 6c 24 70       	mov    0x70(%rsp),%rbp
 1c6:	48 39 ef             	cmp    %rbp,%rdi
 1c9:	0f 84 39 00 00 00    	je     0x208
 1cf:	8b 74 24 68          	mov    0x68(%rsp),%esi
 1d3:	48 83 ee 01          	sub    $0x1,%rsi
 1d7:	48 6b f6 04          	imul   $0x4,%rsi,%rsi
 1db:	48 83 c6 04          	add    $0x4,%rsi
 1df:	48 01 fe             	add    %rdi,%rsi
 1e2:	48 39 ee             	cmp    %rbp,%rsi
 1e5:	0f 86 1d 00 00 00    	jbe    0x208
 1eb:	44 8b 4c 24 68       	mov    0x68(%rsp),%r9d
 1f0:	49 83 e9 01          	sub    $0x1,%r9
 1f4:	4d 6b c9 04          	imul   $0x4,%r9,%r9
 1f8:	49 83 c1 04          	add    $0x4,%r9
 1fc:	49 01 e9             	add    %rbp,%r9
 1ff:	49 39 f9             	cmp    %rdi,%r9
 202:	0f 87 78 fe ff ff    	ja     0x80
 208:	44 8b 4c 24 38       	mov    0x38(%rsp),%r9d
 20d:	66 41 0f 6e c9       	movd   %r9d,%xmm1
 212:	66 0f 70 c9 00       	pshufd $0x0,%xmm1,%xmm1
 217:	44 8b 4c 24 40       	mov    0x40(%rsp),%r9d
 21c:	66 41 0f 6e d1       	movd   %r9d,%xmm2
 221:	66 0f 70 d2 00       	pshufd $0x0,%xmm2,%xmm2
 226:	48 8b 7c 24 78       	mov    0x78(%rsp),%rdi
 22b:	48 8b 74 24 70       	mov    0x70(%rsp),%rsi
 230:	8b 6c 24 68          	mov    0x68(%rsp),%ebp
 234:	83 e5 fc             	and    $0xfffffffc,%ebp
 237:	89 6c 24 68          	mov    %ebp,0x68(%rsp)
 23b:	c1 ed 02             	shr    $0x2,%ebp
 23e:	c4 e1 7a 6f 04 33    	vmovdqu (%rbx,%rsi,1),%xmm0
 244:	0f 59 c1             	mulps  %xmm1,%xmm0
 247:	0f 58 c2             	addps  %xmm2,%xmm0
 24a:	c4 e1 7a 7f 04 3b    	vmovdqu %xmm0,(%rbx,%rdi,1)
 250:	48 83 c7 10          	add    $0x10,%rdi
 254:	48 83 c6 10          	add    $0x10,%rsi
 258:	83 ed 01             	sub    $0x1,%ebp
 25b:	0f 85 dd ff ff ff    	jne    0x23e
 261:	66 0f 70 c0 ff       	pshufd $0xff,%xmm0,%xmm0
 266:	8b 7c 24 28          	mov    0x28(%rsp),%edi
 26a:	8b 74 24 68          	mov    0x68(%rsp),%esi
 26e:	6b f6 04             	imul   $0x4,%esi,%esi
 271:	01 f7                	add    %esi,%edi
 273:	41 89 fc             	mov    %edi,%r12d
 276:	8b 7c 24 30          	mov    0x30(%rsp),%edi
 27a:	8b 74 24 68          	mov    0x68(%rsp),%esi
 27e:	6b f6 04             	imul   $0x4,%esi,%esi
 281:	01 f7                	add    %esi,%edi
 283:	41 89 fd             	mov    %edi,%r13d
 286:	66 0f 7e c7          	movd   %xmm0,%edi
 28a:	66 44 0f 6e e7       	movd   %edi,%xmm12
 28f:	8b 7c 24 50          	mov    0x50(%rsp),%edi
 293:	8b 74 24 68          	mov    0x68(%rsp),%esi
 297:	01 f7                	add    %esi,%edi
 299:	41 89 fe             	mov    %edi,%r14d
 29c:	bf 00 00 00 00       	mov    $0x0,%edi
 2a1:	8b 74 24 58          	mov    0x58(%rsp),%esi
 2a5:	01 f7                	add    %esi,%edi
 2a7:	44 89 f6             	mov    %r14d,%esi
 2aa:	39 f7                	cmp    %esi,%edi
 2ac:	0f 85 ce fd ff ff    	jne    0x80
 2b2:	f3 41 0f 10 c4       	movss  %xmm12,%xmm0
 2b7:	66 0f 7e c7          	movd   %xmm0,%edi
 2bb:	44 89 e6             	mov    %r12d,%esi
 2be:	44 31 ee             	xor    %r13d,%esi
 2c1:	44 89 f5             	mov    %r14d,%ebp
 2c4:	31 f5                	xor    %esi,%ebp
 2c6:	31 ef                	xor    %ebp,%edi
 2c8:	48 89 7c 24 28       	mov    %rdi,0x28(%rsp)
 2cd:	48 8b 44 24 28       	mov    0x28(%rsp),%rax
 2d2:	48 81 c4 88 00 00 00 	add    $0x88,%rsp
 2d9:	c3                   	ret
 2da:	b8 ff ff ff ff       	mov    $0xffffffff,%eax
 2df:	48 8b 73 98          	mov    -0x68(%rbx),%rsi
 2e3:	c7 46 10 01 00 00 00 	movl   $0x1,0x10(%rsi)
 2ea:	89 46 14             	mov    %eax,0x14(%rsi)
 2ed:	c7 06 03 00 00 00    	movl   $0x3,(%rsi)
 2f3:	48 8b 63 e8          	mov    -0x18(%rbx),%rsp
 2f7:	c3                   	ret
