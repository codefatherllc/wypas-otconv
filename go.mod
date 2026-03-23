module github.com/codefatherllc/wypas-otconv

go 1.22.3

require github.com/codefatherllc/wypas-lib v0.0.0

require (
	github.com/codefatherllc/wypas-proto v0.0.0 // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
)

replace github.com/codefatherllc/wypas-lib => ../wypas-lib

replace github.com/codefatherllc/wypas-proto => ../wypas-proto
