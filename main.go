package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-cmd/cmd"
)

func main() {

	if len(os.Args) == 1 {
		usage()
		os.Exit(0)
	}

	// remove command name
	passedArgs := os.Args[1:]

	// check if KUBE_NAMESPACE is set
	namespace, envSet := os.LookupEnv("KUBE_NAMESPACE")
	if envSet {
		// don't use env if flags already specified
		_, foundNS := sliceFind(passedArgs, "--namespace")
		_, foundNS2 := sliceFind(passedArgs, "-n")
		if !foundNS && !foundNS2 {
			passedArgs = append(passedArgs, "--namespace", namespace)
		}
	}
	context, envSet := os.LookupEnv("KUBE_CONTEXT")
	if envSet {
		_, foundContext := sliceFind(passedArgs, "--context")
		if !foundContext {
			passedArgs = append(passedArgs, "--context", context)
		}
	}

	var kubeEnv string

	// check if KUBECONFIG is NOT set
	// we don't set the argument if KUBECONFIG is explicitly set
	_, kubeconfigEnvBool := os.LookupEnv("KUBECONFIG")
	_, kubeconfigArgBool := sliceFind(passedArgs, "--kubeconfig")
	if !kubeconfigEnvBool && !kubeconfigArgBool {
		// if KUBECONFIG isn't set generate one from all the files in ~/.kube
		// ignore cache and http-cache directories
		kubeEnv = buildKubeconfig()
	} else {
		kubeEnv = os.Getenv("KUBECONFIG")
	}

	// check if the first arg is special syntax
	// can specify
	// :namespace
	// @cluster
	// @cluster:namespace
	// user@cluster
	// user@cluster:namespace
	// +context
	// +context:namespace
	if strings.ContainsAny(passedArgs[0], "+@:") {

		clustersMap, kSpaceNames := ParseCluster(passedArgs[0], kubeEnv)
		if len(clustersMap) > 1 {
			// TODO use key name for output
			for name, cluster := range clustersMap {
				args := passedArgs[1:]
				// fmt.Println("Detected multiple args")
				// fmt.Println(clustersMap)
				if cluster.context != "" {
					// only add context because it contains other info
					// parsing doesn't currently support overriding context
					args = append(args, "--context", cluster.context)
				} else {
					fmt.Fprintf(os.Stderr, "failed to process %+v\n", cluster)
					break
				}

				if cluster.namespace != "" {
					if cluster.namespace == "*" {
						args = append(args, "--all-namespaces")
					} else {
						args = append(args, "--namespace", cluster.namespace)
					}
				}

				// fmt.Println(args)
				runKubectl(args, name, kubeEnv)
			}
		} else if len(clustersMap) == 1 {
			// cluster should be of type cluster
			cluster := clustersMap[kSpaceNames[0]]
			// fmt.Println(cluster)
			args := passedArgs[1:]
			if cluster.context != "" {
				// only add context because it contains other info
				// parsing doesn't currently support overriding context
				args = append(args, "--context", cluster.context)
			} else {
				fmt.Fprintf(os.Stderr, "Failed to process %+v\n", cluster)
				os.Exit(1)
			}

			if cluster.namespace != "" {
				if cluster.namespace == "*" {
					args = append(args, "--all-namespaces")
				} else {
					args = append(args, "--namespace", cluster.namespace)
				}
			}

			// fmt.Println(args)
			runKubectl(args, "", kubeEnv)
		}
	} else {
		// fmt.Println(passedArgs)
		runKubectl(passedArgs, "", kubeEnv)
	}
}

func runKubectl(args []string, kspace string, env string) {

	// Disable output buffering, enable streaming
	cmdOptions := cmd.Options{
		Buffered:  false,
		Streaming: true,
	}

	// Create Cmd with options
	kCmd := cmd.NewCmdOptions(cmdOptions, "kubectl", args...)
	kCmd.Env = os.Environ()
	kCmd.Env = append(kCmd.Env, env)

	// Print STDOUT and STDERR lines streaming from Cmd
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		// Done when both channels have been closed
		// https://dave.cheney.net/2013/04/30/curious-channels
		for kCmd.Stdout != nil || kCmd.Stderr != nil {
			select {
			case line, open := <-kCmd.Stdout:
				if !open {
					kCmd.Stdout = nil
					continue
				}
				if kspace != "" {
					fmt.Println(kspace, line)
				} else {
					fmt.Println(line)
				}
			case line, open := <-kCmd.Stderr:
				if !open {
					kCmd.Stderr = nil
					continue
				}
				fmt.Fprintln(os.Stderr, line)
			}
		}
	}()

	// Run and wait for Cmd to return, discard Status
	// fmt.Println(kCmd)
	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		// Check if k was piped to and pass stdin to kubectl
		<-kCmd.StartWithStdin(os.Stdin)
	} else {
		<-kCmd.Start()
	}

	// Wait for goroutine to print everything
	<-doneChan
}

func captureFirst(r *regexp.Regexp, s string) string {
	c := r.FindStringSubmatch(s)
	if len(c) > 1 {
		return c[1]
	}
	return ""
}

// ParseCluster is the main function to parse the first argument given to k
// It attempts to parse the following patterns
// [@cluster][,cluster][:namespace][,namespace]
// [+context][,context][:namespace][,namespace]
func ParseCluster(s string, env string) (map[string]Cluster, []string) {
	kSpace := make(map[string]Cluster)

	var tmpCluster Cluster
	var tmpName string

	var maybeContext []string
	var maybeCluster []string
	var maybeNamespace []string

	// I'm sorry. Regex was the easiest way to parse a string
	maybeContext = strings.Split(captureFirst(regexp.MustCompile(`(?:\+)([0-9A-Za-z_,.\-@]+)`), s), ",") // capture between + and : or $
	maybeCluster = strings.Split(captureFirst(regexp.MustCompile(`(?:@)([0-9A-Za-z_,.\-@]+)`), s), ",")  // capture between @ and : or $
	maybeNamespace = strings.Split(captureFirst(regexp.MustCompile(`(?::)(.+)(?:$)`), s), ",")           // capture between : and $

	// fmt.Println("maybeContext: ", maybeContext, len(maybeContext))
	// fmt.Println("maybeNamespace: ", maybeNamespace, len(maybeNamespace))
	// fmt.Println("maybeCluster: ", maybeCluster, len(maybeCluster))

	if maybeContext[0] != "" {
		// check if we have more than 1 namespace
		if len(maybeNamespace) > 0 && maybeNamespace[0] != "" {
			for _, ctx := range maybeContext {
				for _, ns := range maybeNamespace {
					// run if given 1 or more context and 1 or more namespace
					tmpName = "+" + ctx + ":" + ns
					// fmt.Println(tmpName)
					tmpCluster.context = ctx
					tmpCluster.namespace = ns
					kSpace[tmpName] = tmpCluster
				}
			}
		} else {
			// No namespace given
			for _, ctx := range maybeContext {
				tmpName = "+" + ctx
				tmpCluster.context = ctx
				kSpace[tmpName] = tmpCluster
			}
		}

	} else if maybeCluster[0] != "" {
		if len(maybeNamespace) > 0 && maybeNamespace[0] != "" {
			for _, cl := range maybeCluster {
				for _, ns := range maybeNamespace {
					// run if given 1 or more context and 1 or more namespace
					tmpName = "@" + cl + ":" + ns
					// fmt.Println(tmpName)
					tmpCluster.context = getContextFromCluster(cl, env)
					tmpCluster.cluster = cl
					tmpCluster.namespace = ns
					kSpace[tmpName] = tmpCluster
				}
			}
		} else {
			// No namespace given
			for _, cl := range maybeCluster {
				tmpName = "@" + cl
				tmpCluster.cluster = cl
				tmpCluster.context = getContextFromCluster(cl, env)
				kSpace[tmpName] = tmpCluster
			}
		}
	} else if maybeNamespace[0] != "" {
		for _, ns := range maybeNamespace {
			tmpName = ":" + ns
			tmpCluster.namespace = ns
			kSpace[tmpName] = tmpCluster
		}
	}

	kNames := make([]string, len(kSpace))

	i := 0
	for k := range kSpace {
		kNames[i] = k
		i++
	}
	return kSpace, kNames
}

func getContextFromCluster(s string, env string) string {
	// reads in a cluster string and KUBECONFIG env

	var context string

	envCmd := cmd.NewCmd("kubectl", "config", "get-contexts")
	// fmt.Println(envCmd)
	envCmd.Env = append(envCmd.Env, env)

	// Run and wait for Cmd to return Status
	status := <-envCmd.Start()

	// Print each line of STDOUT from Cmd
	for i, line := range status.Stdout {
		if i > 0 {
			//skip header
			// fmt.Println(strings.Fields(trimFirstRune(line))[1])
			contextMatch, _ := regexp.MatchString(strings.Fields(trimFirstRune(line))[1], s)
			if contextMatch {
				context = strings.Fields(trimFirstRune(line))[0]
				// fmt.Println(context)
			}
		}
	}
	return context
}

func trimFirstRune(s string) string {
	_, i := utf8.DecodeRuneInString(s)
	return s[i:]
}

/*
Cluster describes the basic structure for variables we use
for lookups or arguments to kubectl
*/
type Cluster struct {
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
	return "KUBECONFIG=" + strings.TrimSuffix(kubeconfig, ":")
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
	k [+context][:namespace] <kubectl options>
	k <kubectl options>

k is a wrapper for kubectl that makes using multiple clusters, namespaces,
and contexts easy. The first argument is parsed to check if it
contains sepecial characters to add required flags to kubectl.

Because clusters are often tied to user authentication using the @cluster
shorthand will find the context that contains that cluster and run
kubectl using the correct context instead.

Examples:
	k :kube-public apply -f pod.yaml
	Runs: kubectl apply -f pod.yaml --namespace kube-public

	k +us-east-1 cluster-info
	Runs: kubectl --context us-east-1 cluster-info

	k @prod:kube-system get pods
	Runs: kubectl --context prod --namespace kube-system get pods

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

// TODO: figure out how to handle interrupts when using --watch
// TODO: figure out how to handle exec (especially with multiple commands)
// TODO: align column output
