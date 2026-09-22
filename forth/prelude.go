package forth

// prelude holds the part of the dictionary that is easier to write in Forth
// than in Go. It is evaluated by New.
const prelude = `
8 CONSTANT CELL
: 2+ ( n -- n+2 ) 2 + ;
: 2- ( n -- n-2 ) 2 - ;
: WITHIN ( n lo hi -- flag ) OVER - >R - R> U< ;
: /STRING ( addr u n -- addr+n u-n ) TUCK - >R + R> ;
: BOUNDS ( addr u -- addr+u addr ) OVER + SWAP ;
: BLANK ( addr u -- ) BL FILL ;
: CMOVE ( src dst u -- ) MOVE ;

\ Double cell output, built on pictured numeric output.
: D. ( d -- ) TUCK DABS <# #S ROT SIGN #> TYPE SPACE ;
: D.R ( d width -- ) >R TUCK DABS <# #S ROT SIGN #> R> OVER - SPACES TYPE ;
: UD. ( ud -- ) <# #S #> TYPE SPACE ;
: 2ROT ( d1 d2 d3 -- d2 d3 d1 ) 2>R 2SWAP 2R> 2SWAP ;
`
