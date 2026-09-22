\ pictured.fs - pictured numeric output builds a string right to left.
\ Between <# and #> the words # #S HOLD HOLDS and SIGN add characters.

: .cents ( n -- )               \ 1234 prints as 12.34
  abs 0 <# # # [char] . hold #s #> type ;

: .money ( n -- )
  dup 0< if [char] - emit then
  [char] $ emit .cents ;

: .euro ( n -- )                \ HOLDS adds a whole string
  abs 0 <# s"  EUR" holds # # [char] , hold #s #> type ;

: .row ( n -- ) dup 8 .r 4 spaces .money cr ;

."    cents    dollars" cr
1234 .row
   7 .row
-2500 .row

-1234 .euro cr

\ SIGN takes the sign from a number left below the digits.
: .signed ( n -- ) dup abs 0 <# #s rot sign #> type ;
-42 .signed cr
