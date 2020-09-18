package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	shellPattern *regexp.Regexp
	kubeEnv      = ""
	version      = "devel"
)

func init() {
	shellPattern = regexp.MustCompile(`[^\w@%+=:,./-]`)
	checkKubectlExists()
}

func main() {

	if len(os.Args) == 1 {
		usage()
		os.Exit(0)
	}

	_, kDebugBool := os.LookupEnv("K_DEBUG")

	// remove command name
	var passedArgs []string

	for _, arg := range os.Args[1:] {
		passedArgs = append(passedArgs, arg)
	}

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
	if kDebugBool {
		fmt.Printf("[DEBUG] Arguments passed: %s\n", passedArgs)
		fmt.Printf("[DEBUG] Using KUBECONFIG: %s\n", kubeEnv)
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
	kspacePrefixes := []string{"@", "+", ":"}
	var kspaces []string
	var args []string
	for _, arg := range passedArgs {
		if hasPrefixAny(arg, kspacePrefixes) {
			kspaces = append(kspaces, arg)
		} else {
			args = append(args, arg)
		}
	}
	if len(kspaces) > 0 {

		clustersMap, kSpaceNames := ParseCluster(kspaces)
		if len(clustersMap) > 1 {
			// TODO use key name for output
			for name, cluster := range clustersMap {
				if kDebugBool {
					fmt.Println("[DEBUG] Detected multiple args")
					// fmt.Println(clustersMap)
				}
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

				if kDebugBool {
					fmt.Printf("[DEBUG] Running: kubectl %s\n", strings.Join(args, " "))
				}
				runKubectl(args, name)
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

			if kDebugBool {
				fmt.Printf("[DEBUG] Running: kubectl %s\n", strings.Join(args, " "))
			}
			runKubectl(args, "")
		}
	} else {
		if kDebugBool {
			fmt.Printf("[DEBUG] Running: kubectl %s\n", strings.Join(passedArgs, " "))
		}
		runKubectl(passedArgs, "")
	}
}

func runKubectl(args []string, kspace string) {

	// Create Cmd with options
	kCmd := exec.Command("kubectl", args...)
	// set Env to nil to get Env from parent
	kCmd.Env = nil
	kCmd.Env = append(os.Environ(),
		"KUBECONFIG="+kubeEnv,
	)

	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		// Check if k was piped to and pass stdin to kubectl
		kCmd.Stdin = os.Stdin
	}

	stdout, _ := kCmd.StdoutPipe()
	stderr, _ := kCmd.StderrPipe()
	// Create a scanner which scans r in a line-by-line fashion
	stdoutscanner := bufio.NewScanner(stdout)
	stderrscanner := bufio.NewScanner(stderr)

	// Print STDOUT and STDERR lines streaming from Cmd
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		// Read line by line and process it
		for stdoutscanner.Scan() {
			line := stdoutscanner.Text()
			if kspace != "" {
				fmt.Printf("%s\t%s\n", kspace, line)
			} else {
				fmt.Println(line)
			}
		}
		for stderrscanner.Scan() {
			line := stderrscanner.Text()
			if kspace != "" {
				fmt.Printf("%s\t%s\n", kspace, line)
			} else {
				fmt.Println(line)
			}
		}

		// We're all done, unblock the channel
		doneChan <- struct{}{}

	}()
	err := kCmd.Run()
	if err != nil {
		log.Printf("error: %v", err)
	}
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
// [@cluster][:namespace][,namespace]
// [+context][:namespace][,namespace]
func ParseCluster(kspaces []string) (map[string]Cluster, []string) {
	kSpace := make(map[string]Cluster)

	var tmpCluster Cluster
	var tmpName string

	var maybeContext []string
	var maybeCluster []string
	var maybeNamespace []string

	for _, s := range kspaces {
		// I'm sorry. Regex was the easiest way to parse a string
		maybeContext = strings.Split(captureFirst(regexp.MustCompile(`(?:\+)([0-9A-Za-z_,.\-@]+)`), s), ",") // capture between + and : or $
		maybeCluster = strings.Split(captureFirst(regexp.MustCompile(`(?:@)([0-9A-Za-z_,.\-@]+)`), s), ",")  // capture between @ and : or $
		maybeNamespace = strings.Split(captureFirst(regexp.MustCompile(`(?::)(.+)(?:$)`), s), ",")           // capture between : and $

		_, kDebugBool := os.LookupEnv("K_DEBUG")
		if kDebugBool {
			fmt.Println("[DEBUG] Parsed context(s): ", strings.Join(maybeContext, " "))
			fmt.Println("[DEBUG] Parsed namespace(s): ", strings.Join(maybeNamespace, " "))
			fmt.Println("[DEBUG] Parsed cluster(s): ", strings.Join(maybeCluster, " "))
		}

		if maybeContext[0] != "" {
			// check if we have more than 1 namespace
			if len(maybeNamespace) > 0 && maybeNamespace[0] != "" {
				for _, ctx := range maybeContext {
					for _, ns := range maybeNamespace {
						// run if given 1 or more context and 1 or more namespace
						tmpName = "+" + ctx + ":" + ns
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
						tmpCluster.context = getContextFromCluster(cl)
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
					tmpCluster.context = getContextFromCluster(cl)
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
	}

	kNames := make([]string, len(kSpace))

	i := 0
	for k := range kSpace {
		kNames[i] = k
		i++
	}
	return kSpace, kNames
}

func getContextFromCluster(s string) string {
	// reads in a cluster string

	var context string
	template := "{{ range .contexts  }}{{ printf \"%s %s\\n\" .name .context.cluster }}{{ end  }}"

	// execute self
	ctxCmd := exec.Command("kubectl", "config", "view", "--output", "template", "--template", template)
	ctxCmd.Env = append(os.Environ(),
		"KUBECONFIG="+kubeEnv,
	)
	ctxCmd.Stderr = os.Stderr

	// Run and wait for Cmd to return Status
	ctxOutput, err := ctxCmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := ctxCmd.Start(); err != nil {
		log.Fatal(err)
	}

	_, kDebugBool := os.LookupEnv("K_DEBUG")

	scanner := bufio.NewScanner(ctxOutput)
	for scanner.Scan() {
		line := scanner.Text()
		if kDebugBool {
			fmt.Printf("[DEBUG] Looking for %s in %s\n", s, strings.Fields(line))
		}

		contextMatch, _ := regexp.MatchString(strings.Fields(line)[1], s)
		if contextMatch {
			context = strings.Fields(line)[0]
			if kDebugBool {
				fmt.Printf("[DEBUG] Found context: %s\n", context)
			}
			break
		}
	}
	if err := ctxCmd.Wait(); err != nil {
		log.Fatal(err)
	}
	return context
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

func checkKubectlExists() {
	_, err := exec.LookPath("kubectl")
	if err != nil {
		fmt.Printf("didn't find 'kubectl' executable\n")
		os.Exit(1)
	}
	return
}

func usage() {
	usage := `k - kubectl wrapper for advanced usage

Usage:
	k ( @cluster... | +cluster... )[:namespace[,namespace]] <kubectl options>
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

	k +us-east-1 +us-west-2 cluster-info
	Runs: kubectl --context us-east-1 cluster-info
		  kubectl --context us-west-2 cluster-info

	k @prod:kube-system get pods
	# @cluster will look up the name of a context with "cluster"
	Runs: kubectl --context prod --namespace kube-system get pods

	k :default,kube-system get svc
	Runs: kubectl --namespace default get svc
	      kubectl --namespace kube-system get svc

Environment Variables:
	Setting the flags manually will override the environment variable.
	e.g. KUBE_NAMESPACE=kube-system k get pod -n default
	  This example will get pods in the default namespace.

	K_DEBUG:        troubleshoot k wrapper
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

	To print kubectl help use k help
`
	fmt.Printf("%s", usage)
	fmt.Printf("\tk version: \t%s\n", version)
}

func hasPrefixAny(s string, pslice []string) bool {
	for _, prefix := range pslice {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
