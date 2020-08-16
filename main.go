package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-cmd/cmd"
)

func main() {

	if len(os.Args) == 1 {
		usage()
		os.Exit(0)
	}

	// Disable output buffering, enable streaming
	cmdOptions := cmd.Options{
		Buffered:  false,
		Streaming: true,
	}

	args := os.Args[1:]

	// check if KUBE_NAMESPACE is set
	namespace, envSet := os.LookupEnv("KUBE_NAMESPACE")
	if envSet {
		// don't use env if flags already specified
		_, foundNS := sliceFind(args, "--namespace")
		_, foundNS2 := sliceFind(args, "-n")
		if !foundNS && !foundNS2 {
			args = append(args, "--namespace", namespace)
		}
	}
	context, envSet := os.LookupEnv("KUBE_CONTEXT")
	if envSet {
		_, foundContext := sliceFind(args, "--namespace")
		if !foundContext {
			args = append(args, "--context", context)
		}
	}

	// check if the first arg is special syntax
	// can specify
	// :namespace
	// @cluster
	// @cluster:namespace
	// user@cluster
	// user@cluster:namespace
	// +context
	// +context:namespace <- would be nice if this works
	if strings.ContainsAny(args[0], "+@:") {
		clusterArgs := parseCluster(args[0])
		// fmt.Println(clusterArgs)
		args = args[1:]
		if clusterArgs.context != "" {
			// only add context because it contains other info
			// parsing doesn't currently support overriding context
			args = append(args, "--context", clusterArgs.context)
		} else {
			if clusterArgs.namespace != "" {
				// if clusterArgs.namespace == "*" {
				// 	args = append(args, "--all-namespaces")
				// } else {
				args = append(args, "--namespace", clusterArgs.namespace)
				// }
			}
			if clusterArgs.cluster != "" {
				args = append(args, "--cluster", clusterArgs.cluster)
			}
			if clusterArgs.user != "" {
				args = append(args, "--user", clusterArgs.user)
			}
		}
	}

	// Create Cmd with options
	envCmd := cmd.NewCmdOptions(cmdOptions, "kubectl", args...)

	// check if KUBECONFIG is NOT set
	// we don't set the argument if KUBECONFIG is explicitly set
	_, kubeconfigBool := os.LookupEnv("KUBECONFIG")
	if !kubeconfigBool {
		// if KUBECONFIG isn't set generate one from all the files in ~/.kube
		// ignore cache and http-cache directories
		envCmd.Env = os.Environ()
		envCmd.Env = append(envCmd.Env, "KUBECONFIG="+buildKubeconfig())
	}

	// Print STDOUT and STDERR lines streaming from Cmd
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		// Done when both channels have been closed
		// https://dave.cheney.net/2013/04/30/curious-channels
		for envCmd.Stdout != nil || envCmd.Stderr != nil {
			select {
			case line, open := <-envCmd.Stdout:
				if !open {
					envCmd.Stdout = nil
					continue
				}
				fmt.Println(line)
			case line, open := <-envCmd.Stderr:
				if !open {
					envCmd.Stderr = nil
					continue
				}
				fmt.Fprintln(os.Stderr, line)
			}
		}
	}()

	// Run and wait for Cmd to return, discard Status
	<-envCmd.Start()

	// Wait for goroutine to print everything
	<-doneChan
}

func parseCluster(s string) cluster {
	// [user][@cluster][:namespace]
	// [+context][:namespace]
	var clusterArgs cluster
	var maybeContext []string
	var maybeCluster []string
	var maybeNamespace []string

	// +context and @cluster are mutually exclusive
	if strings.ContainsAny(s, "+") {
		// Don't try to pull out context if there's no +
		maybeContext = strings.FieldsFunc(s, parseContext)
	} else if strings.ContainsAny(s, "@") {
		maybeCluster = strings.FieldsFunc(s, parseClusterName)
	}
	if strings.ContainsAny(s, ":") {
		maybeNamespace = strings.FieldsFunc(s, parseNamespace)
	}

	// fmt.Println("maybeNamespace: ", maybeNamespace)
	// fmt.Println("maybeCluster: ", maybeCluster)

	if len(maybeContext) > 0 {
		// if context is set then no other aruments should be parsed
		clusterArgs.context = maybeContext[0]
	} else {
		if len(maybeNamespace) == 2 {
			clusterArgs.namespace = maybeNamespace[1] // [user@cluster namespace]
		} else if len(maybeNamespace) == 1 {
			clusterArgs.namespace = maybeNamespace[0] // [namespace]
		}

		if len(maybeCluster) == 2 { // [ user cluster]
			clusterArgs.user = maybeCluster[0]
			if strings.Contains(maybeCluster[1], "@") {
				clusterName := strings.FieldsFunc(maybeCluster[1], parseNamespace)
				clusterArgs.cluster = clusterName[0]
			}
		} else if len(maybeCluster) == 1 { // [cluster]
			if strings.Contains(maybeCluster[0], "@") {
				clusterArgs.cluster = maybeCluster[0]
			}
		}
	}
	return clusterArgs
}

func parseContext(r rune) bool {
	return r == '+' || r == ':'
}

func parseNamespace(r rune) bool {
	return r == ':' || r == '@' || r == '+'
}

func parseClusterName(r rune) bool {
	return r == '@' || r == ':'
}

type cluster struct {
	user      string
	cluster   string
	namespace string
	context   string
}

func buildKubeconfig() (kc string) {
	var kubeconfig string

	err := filepath.Walk(os.Getenv("HOME")+"/.kube", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if info.IsDir() && (info.Name() == "cache" || info.Name() == "http-cache") {
			// fmt.Printf("skipping a dir without errors: %+v \n", info.Name())
			return filepath.SkipDir
		}
		// fmt.Printf("visited file or dir: %q\n", path)
		if !info.IsDir() {
			// fmt.Println(info.Name())
			if len(kubeconfig) == 0 {
				// no kubeconfig set yet
				kubeconfig = path
			} else {
				if path != "" {
					kubeconfig = kubeconfig + ":" + path
				}
			}

		}
		return nil
	})
	if err != nil {
		fmt.Printf("error walking the path")
		return
	}
	return strings.TrimSuffix(kubeconfig, ":")
}

// sliceFind takes a slice and looks for an element in it. If found it will
// return it's key, otherwise it will return -1 and a bool of false.
func sliceFind(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func usage() {
	usage := `k - kubectl wrapper for advanced usage

Usage:  k [user][@cluster][:namespace] <kubectl options>
	k [+context] <kubectl options>
	k <kubectl options>

k is a wrapper for kubectl that makes using multiple clusters, namespaces,
contexts, and users easier. The first argument is parsed to check if it
contains sepecial characters to add required flags to kubectl.

Examples:
	k :kube-public apply -f pod.yaml
	Runs: kubectl apply -f pod.yaml --namespace kube-public

	k +us-east-1 cluster-info
	Runs: kubectl --context us-east-1 cluster-info

	k @prod:kube-system get pods
	Runs: kubectl --cluster prod --namespace kube-system get pods

Environment Variables:
	Setting the flags manually will override the environment variable.
	e.g. KUBE_NAMESPACE=kube-system k get pod -n default
	  This example will get pods in the default namespace.

	KUBE_NAMESPACE: sets the --namespace argument
	KUBE_CONTEXT:   sets the --context argument

	KUBECONFIG: Kubeconfig can be set manually in your environment.
	If one is not set then all files in $HOME/.kube/** will be 
	added to the kubeconfig	argument (ignoring cache directories).
	e.g. The below directory struture would result in
	KUBECONFIG=$HOME/.kube/config:$HOME/.kube/eksctl/clusters/cluster1
	$HOME/.kube
	├── config
	└── eksctl/
	    └── clusters/
	        └── cluster1

	WARNING: This CLI is likely to change. Please do not rely on it
	for automation or in scripts.

	WARNING 2: Argument parsing will have unpredictable behavior if
	your contexts or clusters have the characters '@', ':', or '+' in
	their name.

	k version 0.0.1

	To print kubectl help use k --help
`
	fmt.Printf("%s", usage)
}
