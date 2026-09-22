\ doubles.fs - 128 bit arithmetic with double cell numbers.
\ A double occupies two cells, the low half below the high half.

: ud* ( ud n -- ud )       \ unsigned double times single
  tuck * >r um* r> + ;

: 2^ ( n -- ud ) 1 0 rot 0 ?do d2* loop ;

: d-factorial ( n -- ud ) 1 0 rot 1+ 1 ?do i ud* loop ;

." 2^64  = " 64 2^ ud. cr
." 2^100 = " 100 2^ ud. cr
." 25!   = " 25 d-factorial ud. cr

\ M* widens a single cell product, so nothing overflows.
." 3000000000 * 3000000000 = " 3000000000 3000000000 m* ud. cr

." -5 as a double: " -5 s>d d. cr
." and doubled:    " -5 s>d 2dup d+ d. cr
." compared:       " -5 s>d 5 s>d d< . cr
