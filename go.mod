module github.com/codefatherllc/wypas-otconv

go 1.22.3

require (
	github.com/codefatherllc/wypas-lib v1.18.0
	github.com/go-sql-driver/mysql v1.8.1
)

require filippo.io/edwards25519 v1.1.0 // indirect

replace github.com/codefatherllc/wypas-lib => ../wypas-lib
