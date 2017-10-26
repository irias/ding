httpasset - stand-alone binaries with a web server serving files from a zip file that was appended to the binary

httpasset is a library that:
1. Opens the zip file that was appended to a go binary.
2. Create a net/http FileSystem from that zip file, allowing the binary to serve the static files from the binary using net/http's FileServer.

See https://godoc.org/bitbucket.org/mjl/httpasset
