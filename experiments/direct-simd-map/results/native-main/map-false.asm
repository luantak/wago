
experiments/direct-simd-map/results/native-main/map-false.bin:     file format binary


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
  30:	48 81 ec 38 00 00 00 	sub    $0x38,%rsp
  37:	41 89 c4             	mov    %eax,%r12d
  3a:	41 89 cd             	mov    %ecx,%r13d
  3d:	41 89 d6             	mov    %edx,%r14d
  40:	44 89 c5             	mov    %r8d,%ebp
  43:	44 89 4c 24 10       	mov    %r9d,0x10(%rsp)
  48:	44 89 54 24 14       	mov    %r10d,0x14(%rsp)
  4d:	31 c0                	xor    %eax,%eax
  4f:	45 31 db             	xor    %r11d,%r11d
  52:	85 ed                	test   %ebp,%ebp
  54:	0f 84 5c 00 00 00    	je     0xb6
  5a:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  5f:	66 0f 1f 84 00 00 00 	nopw   0x0(%rax,%rax,1)
  66:	00 00 
  68:	45 89 ed             	mov    %r13d,%r13d
  6b:	49 8d 7d 04          	lea    0x4(%r13),%rdi
  6f:	4c 39 ff             	cmp    %r15,%rdi
  72:	0f 87 60 00 00 00    	ja     0xd8
  78:	46 8b 1c 2b          	mov    (%rbx,%r13,1),%r11d
  7c:	44 0f af 5c 24 10    	imul   0x10(%rsp),%r11d
  82:	44 03 5c 24 14       	add    0x14(%rsp),%r11d
  87:	45 89 e4             	mov    %r12d,%r12d
  8a:	49 8d 7c 24 04       	lea    0x4(%r12),%rdi
  8f:	4c 39 ff             	cmp    %r15,%rdi
  92:	0f 87 40 00 00 00    	ja     0xd8
  98:	46 89 1c 23          	mov    %r11d,(%rbx,%r12,1)
  9c:	41 83 c4 04          	add    $0x4,%r12d
  a0:	41 83 c5 04          	add    $0x4,%r13d
  a4:	41 83 c6 01          	add    $0x1,%r14d
  a8:	41 39 ee             	cmp    %ebp,%r14d
  ab:	0f 85 b7 ff ff ff    	jne    0x68
  b1:	0f 1f 44 00 00       	nopl   0x0(%rax,%rax,1)
  b6:	44 89 e7             	mov    %r12d,%edi
  b9:	44 31 ef             	xor    %r13d,%edi
  bc:	44 89 f6             	mov    %r14d,%esi
  bf:	31 fe                	xor    %edi,%esi
  c1:	44 89 df             	mov    %r11d,%edi
  c4:	31 f7                	xor    %esi,%edi
  c6:	48 89 7c 24 20       	mov    %rdi,0x20(%rsp)
  cb:	48 8b 44 24 20       	mov    0x20(%rsp),%rax
  d0:	48 81 c4 38 00 00 00 	add    $0x38,%rsp
  d7:	c3                   	ret
  d8:	b8 ff ff ff ff       	mov    $0xffffffff,%eax
  dd:	48 8b 73 98          	mov    -0x68(%rbx),%rsi
  e1:	c7 46 10 01 00 00 00 	movl   $0x1,0x10(%rsi)
  e8:	89 46 14             	mov    %eax,0x14(%rsi)
  eb:	c7 06 03 00 00 00    	movl   $0x3,(%rsi)
  f1:	48 8b 63 e8          	mov    -0x18(%rbx),%rsp
  f5:	c3                   	ret
