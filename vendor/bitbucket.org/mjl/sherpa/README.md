Sherpa is a library in Go for providing and consuming Sherpa API's.

The Sherpa specification can be found at:

	https://www.ueber.net/who/mjl/sherpa/

This library makes it trivial to export Go functions as a Sherpa API, including documentation.

Use sherpaweb to read API documentation and call methods (for testing purposes).

Use the CLI tool sherpaclient to inspect API's through the command-line:

	# the basics of the API
	$ sherpaclient -info https://sherpa.irias.nl/example/

	# call a function
	$ sherpaclient http://localhost:8080/example/ echo '["test", 123, [], {"a": 123.34, "b": null}]'
	["test",123,[],{"a":123.34,"b":null}]

	# all documentation
	$ sherpaclient -doc https://sherpa.irias.nl/example/
	...

	# documentation for just one function
	$ sherpaclient -doc https://sherpa.irias.nl/example/ echo
	...

Use sherpadoc to generate Sherpa documentation from the comments in the Go source files.

	$ sherpadoc Example >example.json

See https://bitbucket.org/mjl/sherpaweb/ with its Example API for an example.


# Documentation

https://godoc.org/bitbucket.org/mjl/sherpa

# Compiling

	go get bitbucket.org/mjl/sherpa

# About

Written by Mechiel Lukkien, mechiel@ueber.net. Bug fixes, patches, comments are welcome.
MIT-licensed, see LICENSE.


# todo

- on errors in handler functions, it seems we get stack traces that are very long? is this normal?
- check if we need to set more headers, and if cors headers are correct
- allow more fields in error response objects?
- more strict with incoming parameters: error for unrecognized field in objects
- say something about variadic parameters

- handler: write tests
- handler: write documentation
- handler: run jshint on the js code in sherpajs.go

- client: write tests
