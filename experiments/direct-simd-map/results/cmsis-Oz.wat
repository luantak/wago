(module
  (type (;0;) (func))
  (type (;1;) (func (param i32 f32 i32 i32)))
  (type (;2;) (func (param i32 i32)))
  (type (;3;) (func (param i32)))
  (type (;4;) (func (result i32)))
  (table (;0;) 2 2 funcref)
  (memory (;0;) 258 258)
  (global (;0;) (mut i32) i32.const 66576)
  (export "memory" (memory 0))
  (export "arm_scale_f32" (func 1))
  (export "_initialize" (func 0))
  (export "__indirect_function_table" (table 0))
  (export "setThrew" (func 2))
  (export "_emscripten_stack_restore" (func 3))
  (export "emscripten_stack_get_current" (func 4))
  (elem (;0;) (i32.const 1) func 0)
  (func (;0;) (type 0)
    nop
  )
  (func (;1;) (type 1) (param i32 f32 i32 i32)
    loop ;; label = @1
      local.get 3
      if ;; label = @2
        local.get 2
        local.get 1
        local.get 0
        f32.load
        f32.mul
        f32.store
        local.get 3
        i32.const 1
        i32.sub
        local.set 3
        local.get 2
        i32.const 4
        i32.add
        local.set 2
        local.get 0
        i32.const 4
        i32.add
        local.set 0
        br 1 (;@1;)
      end
    end
  )
  (func (;2;) (type 2) (param i32 i32)
    i32.const 1024
    i32.load
    i32.eqz
    if ;; label = @1
      i32.const 1028
      local.get 1
      i32.store
      i32.const 1024
      local.get 0
      i32.store
    end
  )
  (func (;3;) (type 3) (param i32)
    local.get 0
    global.set 0
  )
  (func (;4;) (type 4) (result i32)
    global.get 0
  )
)
