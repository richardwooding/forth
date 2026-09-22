\ defining-words.fs - CREATE ... DOES> builds words that define words.

\ ARRAY defines a word that maps an index to the address of a cell.
: array ( n "name" -- ) create cells allot does> swap cells + ;

10 array squares
: init-squares ( -- ) 10 0 do i dup * i squares ! loop ;
: .squares ( -- ) 10 0 do i squares @ . loop cr ;

init-squares
." squares: " .squares

\ A defining word for scaled constants: the value is compiled into the body
\ and DOES> fetches it.
: hours ( n "name" -- ) create 3600 * , does> @ ;

2 hours two-hours
." two hours is " two-hours . ." seconds" cr

\ CONSTANT and VARIABLE written in Forth, to show what they do.
: my-constant ( n "name" -- ) create , does> @ ;
: my-variable ( "name" -- ) create 0 , ;

7 my-constant seven
my-variable counter
seven counter !
." counter holds " counter ? cr
