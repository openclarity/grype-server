module github.com/Portshift/grype-server/grype-server

go 1.16

require (
	github.com/Portshift/go-utils v0.0.0-20211114210214-d8e30d7d4673
	github.com/Portshift/grype-server/api v0.0.0
	github.com/anchore/grype v0.32.0
	github.com/anchore/syft v0.36.0
	github.com/go-openapi/loads v0.21.0
	github.com/go-openapi/runtime v0.21.0
	github.com/jessevdk/go-flags v1.5.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/viper v1.8.1
	github.com/urfave/cli v1.22.5
)

replace github.com/Portshift/grype-server/api v0.0.0 => ./../api

// some replace to fix high/critical vulnerabilities
// fix github.com/containerd/containerd GHSA-crp2-qrr5-8pq7
// fix github.com/hashicorp/go-getter GHSA-x24g-9w7v-vprh, GHSA-fcgg-rvwg-jv58, GHSA-28r2-q6m8-9hpx, GHSA-cjr4-fv6c-f3mv
// remove them after github.com/anchore/grype and github.com/anchore/syft upgrade
replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.6.1
	github.com/hashicorp/go-getter => github.com/hashicorp/go-getter v1.6.1
)
