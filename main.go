package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-cmd/cmd"
)

func main() {

	// Disable output buffering, enable streaming
	cmdOptions := cmd.Options{
		Buffered:  false,
		Streaming: true,
	}

	args := os.Args[1:]

	// check if KUBECONFIG is NOT set
	// we don't set the argument if KUBECONFIG is explicitly set
	_, envSet := os.LookupEnv("KUBECONFIG")
	if !envSet {
		// if KUBECONFIG isn't set generate one from all the files in ~/.kube
		// ignore cache and http-cache directories
		args = append(args, "--kubeconfig", buildKubeconfig())
	}

	// check if KUBE_NAMESPACE is set
	namespace, envSet := os.LookupEnv("KUBE_NAMESPACE")
	if envSet {
		// don't use env if flags already specified
		_, foundNS := Find(args, "--namespace")
		_, foundNS2 := Find(args, "-n")
		if !foundNS && !foundNS2 {
			args = append(args, "--namespace", namespace)
		}
	}
	context, envSet := os.LookupEnv("KUBE_CONTEXT")
	if envSet {
		_, foundContext := Find(args, "--namespace")
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
				args = append(args, "--namespace", clusterArgs.namespace)
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
	envCmd.Env = env

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
	var clusterArgs cluster
	maybeContext := strings.FieldsFunc(s, parseContext)
	maybeNamespace := strings.FieldsFunc(s, parseNamespace)
	maybeCluster := strings.FieldsFunc(s, parseClusterName)
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

		if len(maybeCluster) == 2 {
			clusterArgs.user = maybeCluster[0]
			if strings.Contains(maybeCluster[1], "@") {
				clusterName := strings.FieldsFunc(maybeCluster[1], parseNamespace)
				clusterArgs.cluster = clusterName[0]
			}
		} else if len(maybeCluster) == 1 {
			if strings.Contains(maybeCluster[0], "@") {
				clusterArgs.cluster = maybeCluster[0]
			}
		}
	}
	return clusterArgs
}

func parseContext(r rune) bool {
	return r == '+'
}

func parseNamespace(r rune) bool {
	return r == ':'
}

func parseClusterName(r rune) bool {
	return r == '@'
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
			if len(kubeconfig) == 0 {
				// no kubeconfig set yet
				kubeconfig = path
			} else {
				kubeconfig = kubeconfig + ":" + path
			}

		}
		return nil
	})
	if err != nil {
		fmt.Printf("error walking the path")
		return
	}
	return kubeconfig
}

// Find takes a slice and looks for an element in it. If found it will
// return it's key, otherwise it will return -1 and a bool of false.
func Find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}
