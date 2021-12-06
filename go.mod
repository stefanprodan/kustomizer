module github.com/stefanprodan/kustomizer

go 1.16

require (
	github.com/fluxcd/pkg/ssa v0.4.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/spf13/cobra v1.1.3
	k8s.io/api v0.22.2
	k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	sigs.k8s.io/cli-utils v0.26.0
	sigs.k8s.io/controller-runtime v0.10.1
	sigs.k8s.io/kustomize/api v0.9.0
	sigs.k8s.io/kustomize/kyaml v0.11.1
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/google/go-cmp => github.com/google/go-cmp v0.5.5
