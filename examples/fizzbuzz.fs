\ fizzbuzz.fs - counted loops, conditionals and early exit.

: divisible? ( n d -- flag ) mod 0= ;

: fizzbuzz ( n -- )
  dup 15 divisible? if ." FizzBuzz" drop exit then
  dup  3 divisible? if ." Fizz"     drop exit then
  dup  5 divisible? if ." Buzz"     drop exit then
  0 .r ;

: fizzbuzz-to ( n -- ) 1+ 1 ?do i fizzbuzz space loop cr ;

20 fizzbuzz-to
