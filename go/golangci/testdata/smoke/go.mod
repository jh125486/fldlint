module example.com/smoke

go 1.26.0

require (
	github.com/launchdarkly/go-sdk-common/v3 v3.0.0
	github.com/launchdarkly/go-server-sdk/v7 v7.0.0
	github.com/open-feature/go-sdk v1.0.0
)

replace (
	github.com/launchdarkly/go-sdk-common/v3 => ../stubs/ldcommon
	github.com/launchdarkly/go-server-sdk/v7 => ../stubs/ldserver
	github.com/open-feature/go-sdk => ../stubs/openfeature
)
