# k

`k` is an experimental wrapper for kubectl.
It does not explicitly take any arguments unless the first argument has 

`k` does not implement any functionallity from `kubectl` but rather adds arguments and makes switching contexts easier for multi-cluster management.
- `KUBE_NAMESPACE` and `KUBE_CONTEXT` will automatically append `--namespace` and `--context` to your `kubectl` command.
- `KUBECONFIG` (if not already set or passed with `--kubeconfig`) is automatically generated from all files in $HOME/.kube directory.
- Shorthand for context (`+`), cluster (`@`), and namespace (`:`) can be used as the first argument for faster context switching. Combine multiple contexts, clusters, and namespaces into a single `k` command with comma separated keywords (see examples).

`k` passes all arguments to `kubectl`.
To print help use `k` by itself.
`kubectl` help output can be printed with `kubectl help`

## Install

Install `k` using brew on macOS and Linux

```
brew install rothgar/tap/k
```

Or you can install with `go get` if you hate having your software automatically updated

```
go get github.com/rothgar/k
```

## Examples

```
KUBE_NAMESPACE=kube-system
k get pods
# RUNS: kubectl get pods --namespace kube-system

k :kube-system get pods
# RUNS: kubectl get pods --namespace kube-system

k +prod get pods --all-namespaces
# RUNS: kubectl get pods --all-namespaces --context prod
```

When you run `k @cluster` it will first run `kubectl get contexts` (using a combined KUBECONFIG if necessary) and find the requested cluster and which context is associated with that cluster.
It will then run `kubectl` with the requested context.
If you have a "test" context that has a "test" cluster
```
k @test get services
# RUNS: kubectl get services --context test
```

Combine contexts or clusters with namespaces
```
k +us-east-1:nginx get pods
# RUNS: kubectl get pods --context us-east-1 --namespace nginx
```

Combine multiple clusters, contexts, and namespaces with commas.
```
k +us-east-1,us-west-2:default:nginx get pods
# RUNS: kubectl get pods --context us-east-1 --namespace default
#       kubectl get pods --context us-east-1 --namespace nginx
#       kubectl get pods --context us-west-2 --namespace default
#       kubectl get pods --context us-west-2 --namespace nginx
```

Or deploy to multiple clusters at once
```
k @kind,dev apply -f deploy.yaml
# RUNS: kubectl apply -f deploy.yaml --context kind
#       kubectl apply -f deploy.yaml --context dev
```

When multiple `kubectl` commands are run all output is combined with a special `KSPACE` variable which represents the arguments provided from the cli.
```
SAMPLE OUTPUT
```

`+context` and `@cluster` are mutually exclusive because context names may have `@` symbols in them.
```
k +prod@test:istio-system get cm
# RUNS: kubectl get cm --context prod@test --namespace istio-system
```

## KUBECONFIG

The `KUBECONFIG` environment is set by walking the $HOME/.kube directory (excluding a couple cache directories) and combining all files into one string.

```
	$HOME/.kube
	├── config
	└── eksctl/
	    └── clusters/
	        └── cluster1
```
Would result in a KUBECONFIG environment variable
`$HOME/.kube/config:$HOME/.kube/eksctl/clusters/cluster1`

When `kubectl` is run it will automatically combine all files into one config and all contexts and clusters will be available.

You can print the combined config with `k config view`.

To not have KUBECONFIG be automatically generated you should export the environment variable to a value (e.g. `export KUBECONFIG=$HOME/.kube/config`)

If you have multiple files be careful which context is set as default or be careful running commands like `k config set-context`.
Default context will be taken from the first file in the list.
Writes to config will happen in the last file in the list.

If you use multiple AWS profiles with `aws-iam-authenticator` make sure you set the AWS_PROFILE variable for each context correctly.
Otherwise you'll get authentication errors.
```
  command: aws-iam-authenticator
  env:
  - name: AWS_STS_REGIONAL_ENDPOINTS
    value: regional
  - name: AWS_DEFAULT_REGION
    value: us-west-2
  - name: AWS_PROFILE
    value: demo
```

## TODO
 - [ ] Support wildcard searching for all kspace arguments
 - [ ] Tab completion for kspace keywords
 - [ ] Support multiple kspaces for `exec` in parallel
 - [x] Support `K_DEBUG` for debug printing