# k

`k` is an experimental wrapper for kubectl.
It does not explicitly take any arguments unless the first argument starts with a special character `+`, `@`, or `:`.

`k` does not change `kubectl` but rather adds arguments (called kspace) and makes switching contexts easier for multi-cluster management.
- Add shorthand for context (`+`), cluster (`@`), and namespace (`:`) can be used for faster context switching. Combine multiple contexts, clusters, and namespaces into a single `k` command (see [examples](#examples)).
- `KUBE_NAMESPACE` and `KUBE_CONTEXT` will automatically append `--namespace` and `--context` to your `kubectl` command.
- `KUBECONFIG` is automatically generated from all files in $HOME/.kube directory if not explicitly set in your environment or passed with `--kubeconfig`.

`k` passes all arguments not prefixed with `@`, `+`, or `:` to `kubectl`.
To print help use `k` by itself.
`kubectl` help output can be printed with `k help`

## Install

Install `k` using brew on macOS and Linux

```
brew install rothgar/tap/k
```

Or you can install with `go get` if you hate having your software automatically updated

```
go get github.com/rothgar/k
```

After you install k you should alias kubectl to k so copy/paste commands will use k for KUBECONFIG and KUBE_* variables.

```
alias kubectl=k
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

When you run `k @cluster` it will first run `kubectl get contexts` (using a combined `KUBECONFIG` if necessary) and find the requested cluster and which context is associated with that cluster.
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
k +us-east-1 +us-west-2:default:nginx get pods
# RUNS: kubectl get pods --context us-east-1
#       kubectl get pods --context us-west-2 --namespace default
#       kubectl get pods --context us-west-2 --namespace nginx
```

Or deploy to multiple clusters at once
```
k @kind @dev apply -f deploy.yaml
# RUNS: kubectl apply -f deploy.yaml --context kind
#       kubectl apply -f deploy.yaml --context dev
```

When multiple `kubectl` commands are run all output is prepended with a `KSPACE` variable which represents the arguments provided from the cli.
```
k @prod:kube-system @stage:kube-system get po
@prod:kube-system   NAME                       READY   STATUS    RESTARTS   AGE
@prod:kube-system   aws-node-5vntp             1/1     Running   0          16d
@prod:kube-system   kube-proxy-w5ppt           1/1     Running   0          16d
...
@stage:kube-system  NAME                                      READY   STATUS             RESTARTS   AGE
@stage:kube-system  aws-node-2m48z                            1/1     Running            0          15d
@stage:kube-system  aws-node-2x77f                            1/1     Running            0          2d8h
...
```

```
# "prod@test" is the name of a context in this command
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

## Troubleshooting

If you find a problem with `k` please try setting `K_DEBUG=1` in your environment and running your command again.

```
K_DEBUG=1 k +stage get po
K_DEBUG=1 k @stage.us-west-2.eksctl.io get po
[DEBUG] Arguments passed: [@stage.us-west-2.eksctl.io get po]
[DEBUG] Using KUBECONFIG: KUBECONFIG=/home/rothgar/.kube/config:/home/rothgar/.kube/eksctl/clusters/fargate:/home/rothgar/.kube/eksctl/clusters/stage
[DEBUG] Parsed context(s):
[DEBUG] Parsed namespace(s):
[DEBUG] Parsed cluster(s):  stage.us-west-2.eksctl.io
[DEBUG] Looking for stage.us-west-2.eksctl.io in [fargate fargate.uw2]
[DEBUG] Looking for stage.us-west-2.eksctl.io in [rothgar@stage.us-west-2.eksctl.io stage.us-west-2.eksctl.io]
[DEBUG] Found context: rothgar@stage.us-west-2.eksctl.io
[DEBUG] Running: kubectl get po --context rothgar@stage.us-west-2.eksctl.io
NAME                                                       READY   STATUS             RESTARTS   AGE
frontend-687b58699c-bqqct                                  1/1     Running            0          3d6h
crashy-0                                                   0/1     CrashLoopBackOff   2107       7d11h
```

## Devel

You can bulid k locally with

```
go build -o k main.go
```

Releases are done with [goreleaser](https://github.com/goreleaser/goreleaser).
Tag a release and then run

```
export GITHUB_TOKEN=XXXXXXXXXXXX
goreleaser r --rm-dist
```