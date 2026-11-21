package flags

import (
	"fmt"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("audit")

var ldClient *ldclient.LDClient

type auditLogger struct{}

func (auditLogger) Log(name string, value bool) { fmt.Println(name, value) }

func Audited() bool {
	client := auditLogger{}
	on, _ := ldClient.BoolVariation("audit-log", ldcontext.New("user"), false)
	client.Log("audit-log", on)
	return on
}
