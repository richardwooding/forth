\ sieve.fs - the sieve of Eratosthenes over a byte array in data space.

1000 constant limit
create flags limit allot

: prime? ( n -- flag ) flags + c@ ;

: strike ( n -- )          \ clear every multiple of n, from n*n upwards
  dup dup *
  begin dup limit < while
    0 over flags + c!
    over +
  repeat 2drop ;

: sieve ( -- )
  flags limit 1 fill
  limit 2 do i prime? if i strike then loop ;

: .primes ( n -- )         \ print the primes below n
  2 ?do i prime? if i . then loop cr ;

: count-primes ( -- n ) 0 limit 2 do i prime? if 1+ then loop ;

sieve
." primes below 100: " 100 .primes
." primes below " limit 0 .r ." : " count-primes . cr
