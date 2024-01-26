# helm-migrate-release
CLI tool to move a single helm release, all releases in a namespace or all releases in a cluster between different helm storage drivers.

## Usage
```
Migrate Helm releases from $HELM_DRIVER to other drivers.

Usage:
  ./helm-migrate-release [flags] subprogram [args]

Subprograms:
  release <release name>
  namespace
  all

  -kubeconfig string
        path to your kubeconfig file
  -max int
        history length to migrate (default 1)
  -namespace string
        namespace containing releases to migrate (default "default")
  -to string
        kind of resource to migrate to (configmap or secret)
```
