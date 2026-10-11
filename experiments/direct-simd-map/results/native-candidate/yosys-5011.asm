
experiments/direct-simd-map/results/native-candidate/yosys-5011.bin:     file format binary


Disassembly of section .data:

0000000000000000 <.data>:
   0:	48 81 ec 88 00 00 00 	sub    $0x88,%rsp
   7:	41 89 c1             	mov    %eax,%r9d
   a:	f3 44 0f 10 e0       	movss  %xmm0,%xmm12
   f:	f3 44 0f 10 e9       	movss  %xmm1,%xmm13
  14:	31 c0                	xor    %eax,%eax
  16:	45 31 ed             	xor    %r13d,%r13d
  19:	45 31 e4             	xor    %r12d,%r12d
  1c:	45 31 d2             	xor    %r10d,%r10d
  1f:	45 31 db             	xor    %r11d,%r11d
  22:	41 8d 79 08          	lea    0x8(%r9),%edi
  26:	48 8d 77 04          	lea    0x4(%rdi),%rsi
  2a:	4c 39 fe             	cmp    %r15,%rsi
  2d:	0f 87 e6 04 00 00    	ja     0x519
  33:	44 8b 2c 3b          	mov    (%rbx,%rdi,1),%r13d
  37:	41 83 fd 01          	cmp    $0x1,%r13d
  3b:	0f 8c 64 01 00 00    	jl     0x1a5
  41:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  46:	41 8d 79 0c          	lea    0xc(%r9),%edi
  4a:	48 8d 77 04          	lea    0x4(%rdi),%rsi
  4e:	4c 39 fe             	cmp    %r15,%rsi
  51:	0f 87 c2 04 00 00    	ja     0x519
  57:	44 8b 24 3b          	mov    (%rbx,%rdi,1),%r12d
  5b:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
  62:	00 00 
  64:	0f 1f 40 00          	nopl   0x0(%rax)
  68:	e9 46 00 00 00       	jmp    0xb3
  6d:	0f 1f 00             	nopl   (%rax)
  70:	45 89 e4             	mov    %r12d,%r12d
  73:	49 8d 7c 24 04       	lea    0x4(%r12),%rdi
  78:	4c 39 ff             	cmp    %r15,%rdi
  7b:	0f 87 98 04 00 00    	ja     0x519
  81:	c4 a1 1a 59 04 23    	vmulss (%rbx,%r12,1),%xmm12,%xmm0
  87:	45 89 e4             	mov    %r12d,%r12d
  8a:	f3 42 0f 11 04 23    	movss  %xmm0,(%rbx,%r12,1)
  90:	41 83 c4 04          	add    $0x4,%r12d
  94:	41 83 c5 ff          	add    $0xffffffff,%r13d
  98:	4c 89 6c 24 28       	mov    %r13,0x28(%rsp)
  9d:	8b 7c 24 28          	mov    0x28(%rsp),%edi
  a1:	85 ff                	test   %edi,%edi
  a3:	0f 85 c7 ff ff ff    	jne    0x70
  a9:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  ae:	e9 f2 00 00 00       	jmp    0x1a5
  b3:	44 89 ef             	mov    %r13d,%edi
  b6:	85 ff                	test   %edi,%edi
  b8:	0f 84 b2 ff ff ff    	je     0x70
  be:	83 ff 10             	cmp    $0x10,%edi
  c1:	0f 82 a9 ff ff ff    	jb     0x70
  c7:	89 7c 24 68          	mov    %edi,0x68(%rsp)
  cb:	44 89 e7             	mov    %r12d,%edi
  ce:	48 89 7c 24 28       	mov    %rdi,0x28(%rsp)
  d3:	66 4c 0f 7e e7       	movq   %xmm12,%rdi
  d8:	48 89 7c 24 30       	mov    %rdi,0x30(%rsp)
  dd:	44 89 ef             	mov    %r13d,%edi
  e0:	48 89 7c 24 38       	mov    %rdi,0x38(%rsp)
  e5:	bf 00 00 00 00       	mov    $0x0,%edi
  ea:	8b 44 24 28          	mov    0x28(%rsp),%eax
  ee:	01 c7                	add    %eax,%edi
  f0:	8b 74 24 68          	mov    0x68(%rsp),%esi
  f4:	48 83 ee 01          	sub    $0x1,%rsi
  f8:	48 6b f6 04          	imul   $0x4,%rsi,%rsi
  fc:	48 83 c6 04          	add    $0x4,%rsi
 100:	48 89 fd             	mov    %rdi,%rbp
 103:	48 01 f5             	add    %rsi,%rbp
 106:	48 b8 00 00 00 00 01 	movabs $0x100000000,%rax
 10d:	00 00 00 
 110:	48 39 c5             	cmp    %rax,%rbp
 113:	0f 87 57 ff ff ff    	ja     0x70
 119:	48 3b ab e0 fe ff ff 	cmp    -0x120(%rbx),%rbp
 120:	0f 87 4a ff ff ff    	ja     0x70
 126:	48 89 7c 24 70       	mov    %rdi,0x70(%rsp)
 12b:	8b 44 24 30          	mov    0x30(%rsp),%eax
 12f:	66 0f 6e c8          	movd   %eax,%xmm1
 133:	66 0f 70 c9 00       	pshufd $0x0,%xmm1,%xmm1
 138:	48 8b 7c 24 70       	mov    0x70(%rsp),%rdi
 13d:	48 8b 74 24 70       	mov    0x70(%rsp),%rsi
 142:	8b 6c 24 68          	mov    0x68(%rsp),%ebp
 146:	83 e5 fc             	and    $0xfffffffc,%ebp
 149:	89 6c 24 68          	mov    %ebp,0x68(%rsp)
 14d:	c1 ed 02             	shr    $0x2,%ebp
 150:	c4 e1 7a 6f 04 33    	vmovdqu (%rbx,%rsi,1),%xmm0
 156:	0f 59 c1             	mulps  %xmm1,%xmm0
 159:	c4 e1 7a 7f 04 3b    	vmovdqu %xmm0,(%rbx,%rdi,1)
 15f:	48 83 c7 10          	add    $0x10,%rdi
 163:	48 83 c6 10          	add    $0x10,%rsi
 167:	83 ed 01             	sub    $0x1,%ebp
 16a:	0f 85 e0 ff ff ff    	jne    0x150
 170:	66 0f 70 c0 ff       	pshufd $0xff,%xmm0,%xmm0
 175:	8b 7c 24 28          	mov    0x28(%rsp),%edi
 179:	8b 74 24 68          	mov    0x68(%rsp),%esi
 17d:	6b f6 04             	imul   $0x4,%esi,%esi
 180:	01 f7                	add    %esi,%edi
 182:	41 89 fc             	mov    %edi,%r12d
 185:	8b 7c 24 38          	mov    0x38(%rsp),%edi
 189:	8b 74 24 68          	mov    0x68(%rsp),%esi
 18d:	6b f6 ff             	imul   $0xffffffff,%esi,%esi
 190:	01 f7                	add    %esi,%edi
 192:	41 89 fd             	mov    %edi,%r13d
 195:	bf 00 00 00 00       	mov    $0x0,%edi
 19a:	44 89 ee             	mov    %r13d,%esi
 19d:	39 f7                	cmp    %esi,%edi
 19f:	0f 85 cb fe ff ff    	jne    0x70
 1a5:	41 8d 79 14          	lea    0x14(%r9),%edi
 1a9:	48 8d 77 04          	lea    0x4(%rdi),%rsi
 1ad:	4c 39 fe             	cmp    %r15,%rsi
 1b0:	0f 87 63 03 00 00    	ja     0x519
 1b6:	44 8b 2c 3b          	mov    (%rbx,%rdi,1),%r13d
 1ba:	41 83 fd 01          	cmp    $0x1,%r13d
 1be:	0f 8c 5b 01 00 00    	jl     0x31f
 1c4:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 1c9:	41 8d 79 18          	lea    0x18(%r9),%edi
 1cd:	48 8d 77 04          	lea    0x4(%rdi),%rsi
 1d1:	4c 39 fe             	cmp    %r15,%rsi
 1d4:	0f 87 3f 03 00 00    	ja     0x519
 1da:	44 8b 24 3b          	mov    (%rbx,%rdi,1),%r12d
 1de:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
 1e5:	00 00 
 1e7:	90                   	nop
 1e8:	e9 40 00 00 00       	jmp    0x22d
 1ed:	0f 1f 00             	nopl   (%rax)
 1f0:	49 8d 7c 24 04       	lea    0x4(%r12),%rdi
 1f5:	4c 39 ff             	cmp    %r15,%rdi
 1f8:	0f 87 1b 03 00 00    	ja     0x519
 1fe:	c4 a1 12 59 04 23    	vmulss (%rbx,%r12,1),%xmm13,%xmm0
 204:	f3 42 0f 11 04 23    	movss  %xmm0,(%rbx,%r12,1)
 20a:	41 83 c4 04          	add    $0x4,%r12d
 20e:	41 83 c5 ff          	add    $0xffffffff,%r13d
 212:	4c 89 6c 24 28       	mov    %r13,0x28(%rsp)
 217:	8b 7c 24 28          	mov    0x28(%rsp),%edi
 21b:	85 ff                	test   %edi,%edi
 21d:	0f 85 cd ff ff ff    	jne    0x1f0
 223:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 228:	e9 f2 00 00 00       	jmp    0x31f
 22d:	44 89 ef             	mov    %r13d,%edi
 230:	85 ff                	test   %edi,%edi
 232:	0f 84 b8 ff ff ff    	je     0x1f0
 238:	83 ff 10             	cmp    $0x10,%edi
 23b:	0f 82 af ff ff ff    	jb     0x1f0
 241:	89 7c 24 68          	mov    %edi,0x68(%rsp)
 245:	44 89 e7             	mov    %r12d,%edi
 248:	48 89 7c 24 28       	mov    %rdi,0x28(%rsp)
 24d:	66 4c 0f 7e ef       	movq   %xmm13,%rdi
 252:	48 89 7c 24 30       	mov    %rdi,0x30(%rsp)
 257:	44 89 ef             	mov    %r13d,%edi
 25a:	48 89 7c 24 38       	mov    %rdi,0x38(%rsp)
 25f:	bf 00 00 00 00       	mov    $0x0,%edi
 264:	8b 44 24 28          	mov    0x28(%rsp),%eax
 268:	01 c7                	add    %eax,%edi
 26a:	8b 74 24 68          	mov    0x68(%rsp),%esi
 26e:	48 83 ee 01          	sub    $0x1,%rsi
 272:	48 6b f6 04          	imul   $0x4,%rsi,%rsi
 276:	48 83 c6 04          	add    $0x4,%rsi
 27a:	48 89 fd             	mov    %rdi,%rbp
 27d:	48 01 f5             	add    %rsi,%rbp
 280:	48 b8 00 00 00 00 01 	movabs $0x100000000,%rax
 287:	00 00 00 
 28a:	48 39 c5             	cmp    %rax,%rbp
 28d:	0f 87 5d ff ff ff    	ja     0x1f0
 293:	48 3b ab e0 fe ff ff 	cmp    -0x120(%rbx),%rbp
 29a:	0f 87 50 ff ff ff    	ja     0x1f0
 2a0:	48 89 7c 24 70       	mov    %rdi,0x70(%rsp)
 2a5:	8b 44 24 30          	mov    0x30(%rsp),%eax
 2a9:	66 0f 6e c8          	movd   %eax,%xmm1
 2ad:	66 0f 70 c9 00       	pshufd $0x0,%xmm1,%xmm1
 2b2:	48 8b 7c 24 70       	mov    0x70(%rsp),%rdi
 2b7:	48 8b 74 24 70       	mov    0x70(%rsp),%rsi
 2bc:	8b 6c 24 68          	mov    0x68(%rsp),%ebp
 2c0:	83 e5 fc             	and    $0xfffffffc,%ebp
 2c3:	89 6c 24 68          	mov    %ebp,0x68(%rsp)
 2c7:	c1 ed 02             	shr    $0x2,%ebp
 2ca:	c4 e1 7a 6f 04 33    	vmovdqu (%rbx,%rsi,1),%xmm0
 2d0:	0f 59 c1             	mulps  %xmm1,%xmm0
 2d3:	c4 e1 7a 7f 04 3b    	vmovdqu %xmm0,(%rbx,%rdi,1)
 2d9:	48 83 c7 10          	add    $0x10,%rdi
 2dd:	48 83 c6 10          	add    $0x10,%rsi
 2e1:	83 ed 01             	sub    $0x1,%ebp
 2e4:	0f 85 e0 ff ff ff    	jne    0x2ca
 2ea:	66 0f 70 c0 ff       	pshufd $0xff,%xmm0,%xmm0
 2ef:	8b 7c 24 28          	mov    0x28(%rsp),%edi
 2f3:	8b 74 24 68          	mov    0x68(%rsp),%esi
 2f7:	6b f6 04             	imul   $0x4,%esi,%esi
 2fa:	01 f7                	add    %esi,%edi
 2fc:	41 89 fc             	mov    %edi,%r12d
 2ff:	8b 7c 24 38          	mov    0x38(%rsp),%edi
 303:	8b 74 24 68          	mov    0x68(%rsp),%esi
 307:	6b f6 ff             	imul   $0xffffffff,%esi,%esi
 30a:	01 f7                	add    %esi,%edi
 30c:	41 89 fd             	mov    %edi,%r13d
 30f:	bf 00 00 00 00       	mov    $0x0,%edi
 314:	44 89 ee             	mov    %r13d,%esi
 317:	39 f7                	cmp    %esi,%edi
 319:	0f 85 d1 fe ff ff    	jne    0x1f0
 31f:	41 8d 79 20          	lea    0x20(%r9),%edi
 323:	48 8d 77 04          	lea    0x4(%rdi),%rsi
 327:	4c 39 fe             	cmp    %r15,%rsi
 32a:	0f 87 e9 01 00 00    	ja     0x519
 330:	44 8b 14 3b          	mov    (%rbx,%rdi,1),%r10d
 334:	41 83 fa 01          	cmp    $0x1,%r10d
 338:	0f 8c d3 01 00 00    	jl     0x511
 33e:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 343:	41 8d 79 24          	lea    0x24(%r9),%edi
 347:	48 8d 77 04          	lea    0x4(%rdi),%rsi
 34b:	4c 39 fe             	cmp    %r15,%rsi
 34e:	0f 87 c5 01 00 00    	ja     0x519
 354:	44 8b 1c 3b          	mov    (%rbx,%rdi,1),%r11d
 358:	41 b9 00 00 00 00    	mov    $0x0,%r9d
 35e:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
 365:	00 00 
 367:	90                   	nop
 368:	43 8d 3c 8b          	lea    (%r11,%r9,4),%edi
 36c:	48 8d 77 04          	lea    0x4(%rdi),%rsi
 370:	4c 39 fe             	cmp    %r15,%rsi
 373:	0f 87 a0 01 00 00    	ja     0x519
 379:	44 8b 24 3b          	mov    (%rbx,%rdi,1),%r12d
 37d:	49 8d 7c 24 08       	lea    0x8(%r12),%rdi
 382:	4c 39 ff             	cmp    %r15,%rdi
 385:	0f 87 8e 01 00 00    	ja     0x519
 38b:	46 8b 6c 23 04       	mov    0x4(%rbx,%r12,1),%r13d
 390:	41 83 fd 01          	cmp    $0x1,%r13d
 394:	0f 8c 65 01 00 00    	jl     0x4ff
 39a:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 39f:	49 8d 7c 24 0c       	lea    0xc(%r12),%rdi
 3a4:	4c 39 ff             	cmp    %r15,%rdi
 3a7:	0f 87 6c 01 00 00    	ja     0x519
 3ad:	42 8b 7c 23 08       	mov    0x8(%rbx,%r12,1),%edi
 3b2:	41 89 fc             	mov    %edi,%r12d
 3b5:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
 3bc:	00 00 
 3be:	66 90                	xchg   %ax,%ax
 3c0:	e9 48 00 00 00       	jmp    0x40d
 3c5:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
 3cc:	00 00 
 3ce:	66 90                	xchg   %ax,%ax
 3d0:	49 8d 7c 24 04       	lea    0x4(%r12),%rdi
 3d5:	4c 39 ff             	cmp    %r15,%rdi
 3d8:	0f 87 3b 01 00 00    	ja     0x519
 3de:	c4 a1 1a 59 04 23    	vmulss (%rbx,%r12,1),%xmm12,%xmm0
 3e4:	f3 42 0f 11 04 23    	movss  %xmm0,(%rbx,%r12,1)
 3ea:	41 83 c4 04          	add    $0x4,%r12d
 3ee:	41 83 c5 ff          	add    $0xffffffff,%r13d
 3f2:	4c 89 6c 24 28       	mov    %r13,0x28(%rsp)
 3f7:	8b 7c 24 28          	mov    0x28(%rsp),%edi
 3fb:	85 ff                	test   %edi,%edi
 3fd:	0f 85 cd ff ff ff    	jne    0x3d0
 403:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 408:	e9 f2 00 00 00       	jmp    0x4ff
 40d:	44 89 ef             	mov    %r13d,%edi
 410:	85 ff                	test   %edi,%edi
 412:	0f 84 b8 ff ff ff    	je     0x3d0
 418:	83 ff 10             	cmp    $0x10,%edi
 41b:	0f 82 af ff ff ff    	jb     0x3d0
 421:	89 7c 24 68          	mov    %edi,0x68(%rsp)
 425:	44 89 e7             	mov    %r12d,%edi
 428:	48 89 7c 24 28       	mov    %rdi,0x28(%rsp)
 42d:	66 4c 0f 7e e7       	movq   %xmm12,%rdi
 432:	48 89 7c 24 30       	mov    %rdi,0x30(%rsp)
 437:	44 89 ef             	mov    %r13d,%edi
 43a:	48 89 7c 24 38       	mov    %rdi,0x38(%rsp)
 43f:	bf 00 00 00 00       	mov    $0x0,%edi
 444:	8b 44 24 28          	mov    0x28(%rsp),%eax
 448:	01 c7                	add    %eax,%edi
 44a:	8b 74 24 68          	mov    0x68(%rsp),%esi
 44e:	48 83 ee 01          	sub    $0x1,%rsi
 452:	48 6b f6 04          	imul   $0x4,%rsi,%rsi
 456:	48 83 c6 04          	add    $0x4,%rsi
 45a:	48 89 fd             	mov    %rdi,%rbp
 45d:	48 01 f5             	add    %rsi,%rbp
 460:	48 b8 00 00 00 00 01 	movabs $0x100000000,%rax
 467:	00 00 00 
 46a:	48 39 c5             	cmp    %rax,%rbp
 46d:	0f 87 5d ff ff ff    	ja     0x3d0
 473:	48 3b ab e0 fe ff ff 	cmp    -0x120(%rbx),%rbp
 47a:	0f 87 50 ff ff ff    	ja     0x3d0
 480:	48 89 7c 24 70       	mov    %rdi,0x70(%rsp)
 485:	8b 44 24 30          	mov    0x30(%rsp),%eax
 489:	66 0f 6e c8          	movd   %eax,%xmm1
 48d:	66 0f 70 c9 00       	pshufd $0x0,%xmm1,%xmm1
 492:	48 8b 7c 24 70       	mov    0x70(%rsp),%rdi
 497:	48 8b 74 24 70       	mov    0x70(%rsp),%rsi
 49c:	8b 6c 24 68          	mov    0x68(%rsp),%ebp
 4a0:	83 e5 fc             	and    $0xfffffffc,%ebp
 4a3:	89 6c 24 68          	mov    %ebp,0x68(%rsp)
 4a7:	c1 ed 02             	shr    $0x2,%ebp
 4aa:	c4 e1 7a 6f 04 33    	vmovdqu (%rbx,%rsi,1),%xmm0
 4b0:	0f 59 c1             	mulps  %xmm1,%xmm0
 4b3:	c4 e1 7a 7f 04 3b    	vmovdqu %xmm0,(%rbx,%rdi,1)
 4b9:	48 83 c7 10          	add    $0x10,%rdi
 4bd:	48 83 c6 10          	add    $0x10,%rsi
 4c1:	83 ed 01             	sub    $0x1,%ebp
 4c4:	0f 85 e0 ff ff ff    	jne    0x4aa
 4ca:	66 0f 70 c0 ff       	pshufd $0xff,%xmm0,%xmm0
 4cf:	8b 7c 24 28          	mov    0x28(%rsp),%edi
 4d3:	8b 74 24 68          	mov    0x68(%rsp),%esi
 4d7:	6b f6 04             	imul   $0x4,%esi,%esi
 4da:	01 f7                	add    %esi,%edi
 4dc:	41 89 fc             	mov    %edi,%r12d
 4df:	8b 7c 24 38          	mov    0x38(%rsp),%edi
 4e3:	8b 74 24 68          	mov    0x68(%rsp),%esi
 4e7:	6b f6 ff             	imul   $0xffffffff,%esi,%esi
 4ea:	01 f7                	add    %esi,%edi
 4ec:	41 89 fd             	mov    %edi,%r13d
 4ef:	bf 00 00 00 00       	mov    $0x0,%edi
 4f4:	44 89 ee             	mov    %r13d,%esi
 4f7:	39 f7                	cmp    %esi,%edi
 4f9:	0f 85 d1 fe ff ff    	jne    0x3d0
 4ff:	41 83 c1 01          	add    $0x1,%r9d
 503:	45 39 d1             	cmp    %r10d,%r9d
 506:	0f 85 5c fe ff ff    	jne    0x368
 50c:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 511:	48 81 c4 88 00 00 00 	add    $0x88,%rsp
 518:	c3                   	ret
 519:	b8 ff ff ff ff       	mov    $0xffffffff,%eax
 51e:	48 8b 73 90          	mov    -0x70(%rbx),%rsi
 522:	48 8b 36             	mov    (%rsi),%rsi
 525:	44 89 36             	mov    %r14d,(%rsi)
 528:	48 8b 73 98          	mov    -0x68(%rbx),%rsi
 52c:	c7 46 10 94 13 00 00 	movl   $0x1394,0x10(%rsi)
 533:	89 46 14             	mov    %eax,0x14(%rsi)
 536:	c7 06 03 00 00 00    	movl   $0x3,(%rsi)
 53c:	48 8b 63 e8          	mov    -0x18(%rbx),%rsp
 540:	c3                   	ret
