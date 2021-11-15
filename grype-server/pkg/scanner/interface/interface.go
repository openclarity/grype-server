package _interface

type Scanner interface {
	Scan(sbom string) (string, error)
}