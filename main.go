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
	"sync"
)

var (
	kubeEnv       = ""
	version       = "devel"
	kubectlBinary = ""
)

func init() {
	log.SetFlags(0)
	_, err := exec.LookPath("kubectl")
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {

	if len(os.Args) == 1 {
		usage()
		os.Exit(0)
	}

	_, kDebugBool := os.LookupEnv("K_DEBUG")
	kubectlBinary, _ := exec.LookPath("kubecolor")

	// if kubectlBinary != "" {
	// kubectlBinary, _ := exec.LookPath("kubectl")
	// }

	// remove command name
	var passedArgs []string

	for _, arg := range os.Args[1:] {
		passedArgs = append(passedArgs, arg)
	}

	// Handle version command - print k version and kubectl version
	if len(passedArgs) > 0 && passedArgs[0] == "version" {
		fmt.Printf("k version: %s\n", version)
		// Continue to kubectl version below
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
			// Interactive commands cannot be run against multiple targets
			if isInteractiveCommand(args) {
				log.Fatalf("Error: Interactive commands (edit, exec -it, attach, etc.) cannot be run against multiple contexts/clusters/namespaces.\nPlease specify only one target.")
			}
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
				runKubectl(args, name, kubectlBinary)
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
			runKubectl(args, "", kubectlBinary)
		}
	} else {
		if kDebugBool {
			fmt.Printf("[DEBUG] Running: kubectl %s\n", strings.Join(passedArgs, " "))
		}
		runKubectl(passedArgs, "", kubectlBinary)
	}
}

// isInteractiveCommand checks if the kubectl command requires TTY access
func isInteractiveCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	interactiveCommands := []string{
		"edit",
		"attach",
	}

	// Check if the first arg is an interactive command
	for _, cmd := range interactiveCommands {
		if args[0] == cmd {
			return true
		}
	}

	// Check for exec with -it or -ti flags
	if args[0] == "exec" {
		for _, arg := range args {
			if arg == "-it" || arg == "-ti" || arg == "-i" || arg == "-t" {
				return true
			}
		}
	}

	// Check for run with -it or -ti flags
	if args[0] == "run" {
		for _, arg := range args {
			if arg == "-it" || arg == "-ti" {
				return true
			}
		}
	}

	return false
}

// isStreamingCommand checks if the kubectl command streams continuous output
func isStreamingCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	// Check for --watch or -w flags
	for _, arg := range args {
		if arg == "--watch" || arg == "-w" {
			return true
		}
	}

	// Check for logs with --follow or -f
	if args[0] == "logs" {
		for _, arg := range args {
			if arg == "--follow" || arg == "-f" {
				return true
			}
		}
	}

	// Check for get with --watch-only
	for _, arg := range args {
		if arg == "--watch-only" {
			return true
		}
	}

	return false
}

func runKubectl(args []string, kspace string, kubectlBinary string) {

	// Create Cmd with options
	kCmd := exec.Command(kubectlBinary, args...)
	// set Env to nil to get Env from parent
	kCmd.Env = nil
	kCmd.Env = append(os.Environ(),
		"KUBECONFIG="+kubeEnv,
	)

	// For interactive commands, directly attach stdin/stdout/stderr
	if isInteractiveCommand(args) {
		kCmd.Stdin = os.Stdin
		kCmd.Stdout = os.Stdout
		kCmd.Stderr = os.Stderr

		err := kCmd.Run()
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				os.Exit(exitError.ExitCode())
			}
		}
		return
	}

	// For streaming commands (--watch, logs -f), use direct I/O copy
	// to avoid buffering delays, but still support kspace prefixing
	if isStreamingCommand(args) && kspace == "" {
		// Direct attachment for streaming without kspace prefix
		kCmd.Stdout = os.Stdout
		kCmd.Stderr = os.Stderr

		fi, _ := os.Stdin.Stat()
		if (fi.Mode() & os.ModeCharDevice) == 0 {
			kCmd.Stdin = os.Stdin
		}

		err := kCmd.Run()
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				os.Exit(exitError.ExitCode())
			}
		}
		return
	}

	// For non-interactive commands, use pipes to allow line prefixing
	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		// Check if k was piped to and pass stdin to kubectl
		kCmd.Stdin = os.Stdin
	}

	stdout, err := kCmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	stderr, err := kCmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	// For streaming with kspace prefix, use line-by-line reading
	// This allows prefixing but may have slight buffering
	if isStreamingCommand(args) {
		// Use io.Copy with a prefix writer for streaming commands with kspace
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			reader := bufio.NewReader(stdout)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				if kspace != "" {
					fmt.Printf("%s\t%s", kspace, line)
				} else {
					fmt.Print(line)
				}
			}
		}()

		go func() {
			defer wg.Done()
			reader := bufio.NewReader(stderr)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				if kspace != "" {
					fmt.Printf("%s\t%s", kspace, line)
				} else {
					fmt.Print(line)
				}
			}
		}()

		if err := kCmd.Start(); err != nil {
			log.Fatal(err)
		}

		wg.Wait()
		kCmd.Wait()
		return
	}

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

	}()
	err = kCmd.Run()
	if err != nil {
		// stderr is already attached so we don't need to print anything here
		// I'm not positive we should exit. If multiple kubectls are run
		// should the entire k command show an error?
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
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
						tmpCluster.context = getContextFromCluster(cl, kubectlBinary)
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
					tmpCluster.context = getContextFromCluster(cl, kubectlBinary)
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

func getContextFromCluster(s string, kubectlBinary string) string {
	// reads in a cluster string

	var context string
	template := "{{ range .contexts  }}{{ printf \"%s %s\\n\" .name .context.cluster }}{{ end  }}"

	// execute self
	ctxCmd := exec.Command(kubectlBinary, "config", "view", "--output", "template", "--template", template)
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
	_, kDebugBool := os.LookupEnv("K_DEBUG")

	// TODO XDG_HOME
	err := filepath.Walk(os.Getenv("HOME")+"/.kube", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if kDebugBool {
			fmt.Printf("Found: %+v \n", info.Name())
		}
		if info.IsDir() && (info.Name() == "cache" ||
			info.Name() == "http-cache" ||
			info.Name() == "kubens") {
			if kDebugBool {
				fmt.Printf("skipping without errors: %+v \n", info.Name())
			}
			return filepath.SkipDir
		} else {
			if !info.IsDir() &&
				info.Name() != "kubectx" &&
				!strings.Contains(info.Name(), ".lock") {
				// fmt.Println("appending" + info.Name())
				if len(kubeconfig) == 0 {
					// no kubeconfig set yet
					kubeconfig = path
				} else {
					if path != "" {
						kubeconfig = kubeconfig + ":" + path
					}
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

func hasPrefixAny(s string, pslice []string) bool {
	for _, prefix := range pslice {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func usage() {
	usage := `k - kubectl wrapper for advanced usage

Usage:
	k ( @cluster... | +context... )[:namespace[,namespace]] <kubectl options>
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
	your contexts or clusters have a colon in their name.

	To print kubectl help use k help
`
	fmt.Printf("%s", usage)
	fmt.Printf("\tk version: \t%s\n", version)
}
