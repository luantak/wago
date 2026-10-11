
experiments/direct-simd-map/results/native-candidate/map-false.bin:     file format binary


Disassembly of section .data:

0000000000000000 <.data>:
   0:	48 89 f3             	mov    %rsi,%rbx
   3:	4c 8b bb e0 fe ff ff 	mov    -0x120(%rbx),%r15
   a:	51                   	push   %rcx
   b:	48 8b 07             	mov    (%rdi),%rax
   e:	48 8b 4f 08          	mov    0x8(%rdi),%rcx
  12:	48 8b 57 10          	mov    0x10(%rdi),%rdx
  16:	4c 8b 47 18          	mov    0x18(%rdi),%r8
  1a:	4c 8b 4f 20          	mov    0x20(%rdi),%r9
  1e:	4c 8b 57 28          	mov    0x28(%rdi),%r10
  22:	e8 09 00 00 00       	call   0x30
  27:	59                   	pop    %rcx
  28:	48 89 01             	mov    %rax,(%rcx)
  2b:	c3                   	ret
  2c:	0f 1f 40 00          	nopl   0x0(%rax)
  30:	48 81 ec 88 00 00 00 	sub    $0x88,%rsp
  37:	41 89 c4             	mov    %eax,%r12d
  3a:	41 89 cd             	mov    %ecx,%r13d
  3d:	41 89 d6             	mov    %edx,%r14d
  40:	44 89 c5             	mov    %r8d,%ebp
  43:	44 89 4c 24 10       	mov    %r9d,0x10(%rsp)
  48:	44 89 54 24 14       	mov    %r10d,0x14(%rsp)
  4d:	31 c0                	xor    %eax,%eax
  4f:	45 31 db             	xor    %r11d,%r11d
  52:	85 ed                	test   %ebp,%ebp
  54:	0f 84 5c 02 00 00    	je     0x2b6
  5a:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  5f:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
  66:	00 00 
  68:	e9 56 00 00 00       	jmp    0xc3
  6d:	0f 1f 00             	nopl   (%rax)
  70:	45 89 ed             	mov    %r13d,%r13d
  73:	49 8d 7d 04          	lea    0x4(%r13),%rdi
  77:	4c 39 ff             	cmp    %r15,%rdi
  7a:	0f 87 58 02 00 00    	ja     0x2d8
  80:	46 8b 1c 2b          	mov    (%rbx,%r13,1),%r11d
  84:	44 0f af 5c 24 10    	imul   0x10(%rsp),%r11d
  8a:	44 03 5c 24 14       	add    0x14(%rsp),%r11d
  8f:	45 89 e4             	mov    %r12d,%r12d
  92:	49 8d 7c 24 04       	lea    0x4(%r12),%rdi
  97:	4c 39 ff             	cmp    %r15,%rdi
  9a:	0f 87 38 02 00 00    	ja     0x2d8
  a0:	46 89 1c 23          	mov    %r11d,(%rbx,%r12,1)
  a4:	41 83 c4 04          	add    $0x4,%r12d
  a8:	41 83 c5 04          	add    $0x4,%r13d
  ac:	41 83 c6 01          	add    $0x1,%r14d
  b0:	41 39 ee             	cmp    %ebp,%r14d
  b3:	0f 85 b7 ff ff ff    	jne    0x70
  b9:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  be:	e9 f3 01 00 00       	jmp    0x2b6
  c3:	89 ef                	mov    %ebp,%edi
  c5:	44 89 f6             	mov    %r14d,%esi
  c8:	39 f7                	cmp    %esi,%edi
  ca:	0f 86 a0 ff ff ff    	jbe    0x70
  d0:	29 f7                	sub    %esi,%edi
  d2:	83 ff 10             	cmp    $0x10,%edi
  d5:	0f 82 95 ff ff ff    	jb     0x70
  db:	89 7c 24 60          	mov    %edi,0x60(%rsp)
  df:	44 89 e7             	mov    %r12d,%edi
  e2:	48 89 7c 24 20       	mov    %rdi,0x20(%rsp)
  e7:	44 89 ef             	mov    %r13d,%edi
  ea:	48 89 7c 24 28       	mov    %rdi,0x28(%rsp)
  ef:	8b 7c 24 10          	mov    0x10(%rsp),%edi
  f3:	48 89 7c 24 30       	mov    %rdi,0x30(%rsp)
  f8:	8b 7c 24 14          	mov    0x14(%rsp),%edi
  fc:	48 89 7c 24 38       	mov    %rdi,0x38(%rsp)
 101:	44 89 df             	mov    %r11d,%edi
 104:	48 89 7c 24 40       	mov    %rdi,0x40(%rsp)
 109:	44 89 f7             	mov    %r14d,%edi
 10c:	48 89 7c 24 48       	mov    %rdi,0x48(%rsp)
 111:	89 ef                	mov    %ebp,%edi
 113:	48 89 7c 24 50       	mov    %rdi,0x50(%rsp)
 118:	bf 00 00 00 00       	mov    $0x0,%edi
 11d:	44 8b 54 24 28       	mov    0x28(%rsp),%r10d
 122:	44 01 d7             	add    %r10d,%edi
 125:	8b 74 24 60          	mov    0x60(%rsp),%esi
 129:	48 83 ee 01          	sub    $0x1,%rsi
 12d:	48 6b f6 04          	imul   $0x4,%rsi,%rsi
 131:	48 83 c6 04          	add    $0x4,%rsi
 135:	49 89 f9             	mov    %rdi,%r9
 138:	49 01 f1             	add    %rsi,%r9
 13b:	49 ba 00 00 00 00 01 	movabs $0x100000000,%r10
 142:	00 00 00 
 145:	4d 39 d1             	cmp    %r10,%r9
 148:	0f 87 22 ff ff ff    	ja     0x70
 14e:	4c 3b 8b e0 fe ff ff 	cmp    -0x120(%rbx),%r9
 155:	0f 87 15 ff ff ff    	ja     0x70
 15b:	48 89 7c 24 68       	mov    %rdi,0x68(%rsp)
 160:	bf 00 00 00 00       	mov    $0x0,%edi
 165:	44 8b 54 24 20       	mov    0x20(%rsp),%r10d
 16a:	44 01 d7             	add    %r10d,%edi
 16d:	8b 74 24 60          	mov    0x60(%rsp),%esi
 171:	48 83 ee 01          	sub    $0x1,%rsi
 175:	48 6b f6 04          	imul   $0x4,%rsi,%rsi
 179:	48 83 c6 04          	add    $0x4,%rsi
 17d:	49 89 f9             	mov    %rdi,%r9
 180:	49 01 f1             	add    %rsi,%r9
 183:	49 ba 00 00 00 00 01 	movabs $0x100000000,%r10
 18a:	00 00 00 
 18d:	4d 39 d1             	cmp    %r10,%r9
 190:	0f 87 da fe ff ff    	ja     0x70
 196:	4c 3b 8b e0 fe ff ff 	cmp    -0x120(%rbx),%r9
 19d:	0f 87 cd fe ff ff    	ja     0x70
 1a3:	48 89 7c 24 70       	mov    %rdi,0x70(%rsp)
 1a8:	48 8b 7c 24 70       	mov    0x70(%rsp),%rdi
 1ad:	4c 8b 4c 24 68       	mov    0x68(%rsp),%r9
 1b2:	4c 39 cf             	cmp    %r9,%rdi
 1b5:	0f 84 39 00 00 00    	je     0x1f4
 1bb:	8b 74 24 60          	mov    0x60(%rsp),%esi
 1bf:	48 83 ee 01          	sub    $0x1,%rsi
 1c3:	48 6b f6 04          	imul   $0x4,%rsi,%rsi
 1c7:	48 83 c6 04          	add    $0x4,%rsi
 1cb:	48 01 fe             	add    %rdi,%rsi
 1ce:	4c 39 ce             	cmp    %r9,%rsi
 1d1:	0f 86 1d 00 00 00    	jbe    0x1f4
 1d7:	44 8b 54 24 60       	mov    0x60(%rsp),%r10d
 1dc:	49 83 ea 01          	sub    $0x1,%r10
 1e0:	4d 6b d2 04          	imul   $0x4,%r10,%r10
 1e4:	49 83 c2 04          	add    $0x4,%r10
 1e8:	4d 01 ca             	add    %r9,%r10
 1eb:	49 39 fa             	cmp    %rdi,%r10
 1ee:	0f 87 7c fe ff ff    	ja     0x70
 1f4:	41 ba 00 00 00 00    	mov    $0x0,%r10d
 1fa:	44 8b 4c 24 30       	mov    0x30(%rsp),%r9d
 1ff:	45 01 ca             	add    %r9d,%r10d
 202:	66 41 0f 6e ca       	movd   %r10d,%xmm1
 207:	66 0f 70 c9 00       	pshufd $0x0,%xmm1,%xmm1
 20c:	41 ba 00 00 00 00    	mov    $0x0,%r10d
 212:	44 8b 4c 24 38       	mov    0x38(%rsp),%r9d
 217:	45 01 ca             	add    %r9d,%r10d
 21a:	66 41 0f 6e d2       	movd   %r10d,%xmm2
 21f:	66 0f 70 d2 00       	pshufd $0x0,%xmm2,%xmm2
 224:	48 8b 7c 24 70       	mov    0x70(%rsp),%rdi
 229:	48 8b 74 24 68       	mov    0x68(%rsp),%rsi
 22e:	44 8b 4c 24 60       	mov    0x60(%rsp),%r9d
 233:	41 83 e1 fc          	and    $0xfffffffc,%r9d
 237:	44 89 4c 24 60       	mov    %r9d,0x60(%rsp)
 23c:	41 c1 e9 02          	shr    $0x2,%r9d
 240:	c4 e1 7a 6f 04 33    	vmovdqu (%rbx,%rsi,1),%xmm0
 246:	66 0f 38 40 c1       	pmulld %xmm1,%xmm0
 24b:	66 0f fe c2          	paddd  %xmm2,%xmm0
 24f:	c4 e1 7a 7f 04 3b    	vmovdqu %xmm0,(%rbx,%rdi,1)
 255:	48 83 c7 10          	add    $0x10,%rdi
 259:	48 83 c6 10          	add    $0x10,%rsi
 25d:	41 83 e9 01          	sub    $0x1,%r9d
 261:	0f 85 d9 ff ff ff    	jne    0x240
 267:	66 0f 70 c0 ff       	pshufd $0xff,%xmm0,%xmm0
 26c:	8b 7c 24 20          	mov    0x20(%rsp),%edi
 270:	8b 74 24 60          	mov    0x60(%rsp),%esi
 274:	6b f6 04             	imul   $0x4,%esi,%esi
 277:	01 f7                	add    %esi,%edi
 279:	41 89 fc             	mov    %edi,%r12d
 27c:	8b 7c 24 28          	mov    0x28(%rsp),%edi
 280:	8b 74 24 60          	mov    0x60(%rsp),%esi
 284:	6b f6 04             	imul   $0x4,%esi,%esi
 287:	01 f7                	add    %esi,%edi
 289:	41 89 fd             	mov    %edi,%r13d
 28c:	66 0f 7e c7          	movd   %xmm0,%edi
 290:	41 89 fb             	mov    %edi,%r11d
 293:	8b 7c 24 48          	mov    0x48(%rsp),%edi
 297:	8b 74 24 60          	mov    0x60(%rsp),%esi
 29b:	01 f7                	add    %esi,%edi
 29d:	41 89 fe             	mov    %edi,%r14d
 2a0:	bf 00 00 00 00       	mov    $0x0,%edi
 2a5:	8b 74 24 50          	mov    0x50(%rsp),%esi
 2a9:	01 f7                	add    %esi,%edi
 2ab:	44 89 f6             	mov    %r14d,%esi
 2ae:	39 f7                	cmp    %esi,%edi
 2b0:	0f 85 ba fd ff ff    	jne    0x70
 2b6:	44 89 e7             	mov    %r12d,%edi
 2b9:	44 31 ef             	xor    %r13d,%edi
 2bc:	44 89 f6             	mov    %r14d,%esi
 2bf:	31 fe                	xor    %edi,%esi
 2c1:	44 89 df             	mov    %r11d,%edi
 2c4:	31 f7                	xor    %esi,%edi
 2c6:	48 89 7c 24 20       	mov    %rdi,0x20(%rsp)
 2cb:	48 8b 44 24 20       	mov    0x20(%rsp),%rax
 2d0:	48 81 c4 88 00 00 00 	add    $0x88,%rsp
 2d7:	c3                   	ret
 2d8:	b8 ff ff ff ff       	mov    $0xffffffff,%eax
 2dd:	48 8b 73 98          	mov    -0x68(%rbx),%rsi
 2e1:	c7 46 10 01 00 00 00 	movl   $0x1,0x10(%rsi)
 2e8:	89 46 14             	mov    %eax,0x14(%rsi)
 2eb:	c7 06 03 00 00 00    	movl   $0x3,(%rsi)
 2f1:	48 8b 63 e8          	mov    -0x18(%rbx),%rsp
 2f5:	c3                   	ret
