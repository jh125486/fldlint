package flags

import (
	"fmt"
	"time"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	ld "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("theme")

func DarkMode() bool {
	ldClient, err := ld.MakeClient("sdk-key", 5*time.Second)
	if err != nil {
		fmt.Println("init error:", err)
		return false
	}
	defer ldClient.Close()
	on, _ := ldClient.BoolVariation("dark-mode", ldcontext.New("user"), false)
	return on
}
