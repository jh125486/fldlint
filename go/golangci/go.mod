module github.com/jh125486/fldlint/go/golangci

go 1.26.0

require (
	github.com/jh125486/fldlint/go v0.0.0
	golang.org/x/tools v0.53.0
)

require github.com/golangci/plugin-module-register v0.1.2

replace github.com/jh125486/fldlint/go => ../
