\ case.fs - CASE ... OF ... ENDOF ... ENDCASE selects on a value.
\ The value is left on the stack for the default branch, and ENDCASE drops it.

: day-name ( n -- )
  case
    1 of ." Monday"    endof
    2 of ." Tuesday"   endof
    3 of ." Wednesday" endof
    4 of ." Thursday"  endof
    5 of ." Friday"    endof
    6 of ." Saturday"  endof
    7 of ." Sunday"    endof
    ." day " dup 0 .r ." ?"
  endcase ;

: week ( -- ) 8 1 do i day-name cr loop ;

week
9 day-name cr
