package _interface

type Scanner interface {
	Scan(sbom []byte) ([]byte, error)
}
