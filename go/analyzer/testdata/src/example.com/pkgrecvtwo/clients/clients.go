// Package clients holds package-level LaunchDarkly clients.
package clients

import "github.com/launchdarkly/go-server-sdk/v7"

// LD is a client.
var LD *ldclient.LDClient

// H holds a client in a field.
var H struct{ C *ldclient.LDClient }
