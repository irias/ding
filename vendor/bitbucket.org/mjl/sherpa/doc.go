// Package sherpa is a server and client library for Sherpa API's.
//
// Sherpa API's are similar to JSON-RPC, but discoverable and self-documenting.
// Sherpa is defined at https://www.ueber.net/who/mjl/sherpa/.
//
// Use sherpa.NewHandler to export Go functions using a http.Handler.
// An example of how to use NewHandler can be found in https://bitbucket.org/mjl/sherpaweb/
//
// sherpa.NewClient creates an API client for calling remote functions.
package sherpa
