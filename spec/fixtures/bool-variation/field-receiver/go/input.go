package flags

import (

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-server-sdk/v7"
	"github.com/open-feature/go-sdk/openfeature"
)

var client = openfeature.NewClient("api")

type service struct {
	ld *ldclient.LDClient
}

func (s *service) APIEnabled() bool {
	on, _ := s.ld.BoolVariation("new-api", ldcontext.New("user"), false)
	return on
}
