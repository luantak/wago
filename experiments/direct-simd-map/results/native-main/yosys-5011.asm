
experiments/direct-simd-map/results/native-main/yosys-5011.bin:     file format binary


Disassembly of section .data:

0000000000000000 <.data>:
   0:	48 81 ec 38 00 00 00 	sub    $0x38,%rsp
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
  2d:	0f 87 df 01 00 00    	ja     0x212
  33:	44 8b 2c 3b          	mov    (%rbx,%rdi,1),%r13d
  37:	41 83 fd 01          	cmp    $0x1,%r13d
  3b:	0f 8c 65 00 00 00    	jl     0xa6
  41:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  46:	41 8d 79 0c          	lea    0xc(%r9),%edi
  4a:	48 8d 77 04          	lea    0x4(%rdi),%rsi
  4e:	4c 39 fe             	cmp    %r15,%rsi
  51:	0f 87 bb 01 00 00    	ja     0x212
  57:	44 8b 24 3b          	mov    (%rbx,%rdi,1),%r12d
  5b:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
  62:	00 00 
  64:	0f 1f 40 00          	nopl   0x0(%rax)
  68:	45 89 e4             	mov    %r12d,%r12d
  6b:	49 8d 7c 24 04       	lea    0x4(%r12),%rdi
  70:	4c 39 ff             	cmp    %r15,%rdi
  73:	0f 87 99 01 00 00    	ja     0x212
  79:	c4 a1 1a 59 04 23    	vmulss (%rbx,%r12,1),%xmm12,%xmm0
  7f:	45 89 e4             	mov    %r12d,%r12d
  82:	f3 42 0f 11 04 23    	movss  %xmm0,(%rbx,%r12,1)
  88:	41 83 c4 04          	add    $0x4,%r12d
  8c:	41 83 c5 ff          	add    $0xffffffff,%r13d
  90:	4c 89 6c 24 28       	mov    %r13,0x28(%rsp)
  95:	8b 7c 24 28          	mov    0x28(%rsp),%edi
  99:	85 ff                	test   %edi,%edi
  9b:	0f 85 c7 ff ff ff    	jne    0x68
  a1:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  a6:	41 8d 79 14          	lea    0x14(%r9),%edi
  aa:	48 8d 77 04          	lea    0x4(%rdi),%rsi
  ae:	4c 39 fe             	cmp    %r15,%rsi
  b1:	0f 87 5b 01 00 00    	ja     0x212
  b7:	44 8b 2c 3b          	mov    (%rbx,%rdi,1),%r13d
  bb:	41 83 fd 01          	cmp    $0x1,%r13d
  bf:	0f 8c 5b 00 00 00    	jl     0x120
  c5:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  ca:	41 8d 79 18          	lea    0x18(%r9),%edi
  ce:	48 8d 77 04          	lea    0x4(%rdi),%rsi
  d2:	4c 39 fe             	cmp    %r15,%rsi
  d5:	0f 87 37 01 00 00    	ja     0x212
  db:	44 8b 24 3b          	mov    (%rbx,%rdi,1),%r12d
  df:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
  e6:	00 00 
  e8:	49 8d 7c 24 04       	lea    0x4(%r12),%rdi
  ed:	4c 39 ff             	cmp    %r15,%rdi
  f0:	0f 87 1c 01 00 00    	ja     0x212
  f6:	c4 a1 12 59 04 23    	vmulss (%rbx,%r12,1),%xmm13,%xmm0
  fc:	f3 42 0f 11 04 23    	movss  %xmm0,(%rbx,%r12,1)
 102:	41 83 c4 04          	add    $0x4,%r12d
 106:	41 83 c5 ff          	add    $0xffffffff,%r13d
 10a:	4c 89 6c 24 28       	mov    %r13,0x28(%rsp)
 10f:	8b 7c 24 28          	mov    0x28(%rsp),%edi
 113:	85 ff                	test   %edi,%edi
 115:	0f 85 cd ff ff ff    	jne    0xe8
 11b:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 120:	41 8d 79 20          	lea    0x20(%r9),%edi
 124:	48 8d 77 04          	lea    0x4(%rdi),%rsi
 128:	4c 39 fe             	cmp    %r15,%rsi
 12b:	0f 87 e1 00 00 00    	ja     0x212
 131:	44 8b 14 3b          	mov    (%rbx,%rdi,1),%r10d
 135:	41 83 fa 01          	cmp    $0x1,%r10d
 139:	0f 8c cb 00 00 00    	jl     0x20a
 13f:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 144:	41 8d 79 24          	lea    0x24(%r9),%edi
 148:	48 8d 77 04          	lea    0x4(%rdi),%rsi
 14c:	4c 39 fe             	cmp    %r15,%rsi
 14f:	0f 87 bd 00 00 00    	ja     0x212
 155:	44 8b 1c 3b          	mov    (%rbx,%rdi,1),%r11d
 159:	41 b9 00 00 00 00    	mov    $0x0,%r9d
 15f:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
 166:	00 00 
 168:	43 8d 3c 8b          	lea    (%r11,%r9,4),%edi
 16c:	48 8d 77 04          	lea    0x4(%rdi),%rsi
 170:	4c 39 fe             	cmp    %r15,%rsi
 173:	0f 87 99 00 00 00    	ja     0x212
 179:	44 8b 24 3b          	mov    (%rbx,%rdi,1),%r12d
 17d:	49 8d 7c 24 08       	lea    0x8(%r12),%rdi
 182:	4c 39 ff             	cmp    %r15,%rdi
 185:	0f 87 87 00 00 00    	ja     0x212
 18b:	46 8b 6c 23 04       	mov    0x4(%rbx,%r12,1),%r13d
 190:	41 83 fd 01          	cmp    $0x1,%r13d
 194:	0f 8c 5e 00 00 00    	jl     0x1f8
 19a:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 19f:	49 8d 7c 24 0c       	lea    0xc(%r12),%rdi
 1a4:	4c 39 ff             	cmp    %r15,%rdi
 1a7:	0f 87 65 00 00 00    	ja     0x212
 1ad:	42 8b 7c 23 08       	mov    0x8(%rbx,%r12,1),%edi
 1b2:	41 89 fc             	mov    %edi,%r12d
 1b5:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
 1bc:	00 00 
 1be:	66 90                	xchg   %ax,%ax
 1c0:	49 8d 7c 24 04       	lea    0x4(%r12),%rdi
 1c5:	4c 39 ff             	cmp    %r15,%rdi
 1c8:	0f 87 44 00 00 00    	ja     0x212
 1ce:	c4 a1 1a 59 04 23    	vmulss (%rbx,%r12,1),%xmm12,%xmm0
 1d4:	f3 42 0f 11 04 23    	movss  %xmm0,(%rbx,%r12,1)
 1da:	41 83 c4 04          	add    $0x4,%r12d
 1de:	41 83 c5 ff          	add    $0xffffffff,%r13d
 1e2:	4c 89 6c 24 28       	mov    %r13,0x28(%rsp)
 1e7:	8b 7c 24 28          	mov    0x28(%rsp),%edi
 1eb:	85 ff                	test   %edi,%edi
 1ed:	0f 85 cd ff ff ff    	jne    0x1c0
 1f3:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 1f8:	41 83 c1 01          	add    $0x1,%r9d
 1fc:	45 39 d1             	cmp    %r10d,%r9d
 1ff:	0f 85 63 ff ff ff    	jne    0x168
 205:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
 20a:	48 81 c4 38 00 00 00 	add    $0x38,%rsp
 211:	c3                   	ret
 212:	b8 ff ff ff ff       	mov    $0xffffffff,%eax
 217:	48 8b 73 90          	mov    -0x70(%rbx),%rsi
 21b:	48 8b 36             	mov    (%rsi),%rsi
 21e:	44 89 36             	mov    %r14d,(%rsi)
 221:	48 8b 73 98          	mov    -0x68(%rbx),%rsi
 225:	c7 46 10 94 13 00 00 	movl   $0x1394,0x10(%rsi)
 22c:	89 46 14             	mov    %eax,0x14(%rsi)
 22f:	c7 06 03 00 00 00    	movl   $0x3,(%rsi)
 235:	48 8b 63 e8          	mov    -0x18(%rbx),%rsp
 239:	c3                   	ret
