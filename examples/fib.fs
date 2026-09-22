\ fib.fs - the same function written twice: recursively and iteratively.
\ A definition cannot call itself by name, because its name is not visible
\ until the closing ; - RECURSE compiles the call instead.

: fib-rec ( n -- fib )
  dup 2 < if exit then
  1- dup recurse swap 1- recurse + ;

: fib-iter ( n -- fib )
  0 1 rot 0 ?do tuck + loop drop ;

: .fibs ( n -- ) 1+ 0 ?do i fib-iter . loop cr ;

." the first 16 Fibonacci numbers:" cr
15 .fibs
." fib 20, recursively: " 20 fib-rec . cr
." fib 90, iteratively: " 90 fib-iter . cr
