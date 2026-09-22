\ file-io.fs - the file access words. This writes notes.txt in the current
\ directory, reads it back and deletes it again.

: check ( ior -- ) abort" file error" ;

s" notes.txt" w/o create-file check constant out
: put ( addr u -- ) out write-line check ;

s" the quick brown fox" put
s" jumps over the lazy dog" put
out close-file check

s" notes.txt" r/o open-file check constant in

: .line ( -- flag )
  pad 200 in read-line check     ( u flag )
  dup if swap pad swap type cr else swap drop then ;

: cat ( -- ) begin .line 0= until ;

." notes.txt says:" cr
cat
in close-file check

s" notes.txt" delete-file check
." and it is gone again" cr
