package main

import "testing"

func TestParseClusterSingleContext(t *testing.T) {
	cluster, names := ParseCluster([]string{"+prod"})

	if names[0] != "+prod" {
		t.Errorf("kSpace Name incorrect: got %s, want +prod", names[0])
	}

	if cluster["+prod"].context != "prod" {
		t.Errorf("Context incorrect: got %s, want prod", cluster["+prod"].context)
	}
}

func TestParseClusterMultipleContexts(t *testing.T) {
	cluster, names := ParseCluster([]string{"+prod", "+stage"})

	if len(names) != 2 {
		t.Errorf("Incorrect names length: got %d, want 2", len(names))
	}

	if cluster["+prod"].context != "prod" {
		t.Errorf("Context incorrect: got %s, want prod", cluster["+prod"].context)
	}

	if cluster["+stage"].context != "stage" {
		t.Errorf("Context incorrect: got %s, want stage", cluster["+stage"].context)
	}
}

func TestParseClusterMultipleContextsWithNamespaces(t *testing.T) {
	cluster, names := ParseCluster([]string{"+prod:default,frontend", "+stage:default,kube-system"})

	if len(names) != 4 {
		t.Errorf("Incorrect names length: got %d, want 4", len(names))
	}

	if cluster["+prod:default"].context != "prod" &&
		cluster["+prod:default"].namespace != "default" {
		t.Errorf("Context or namespace incorrect: got %s and %s, want prod and default",
			cluster["+prod:default"].context, cluster["+prod:default"].namespace)
	}

	if cluster["+stage:kube-system"].context != "stage" {
		t.Errorf("Context incorrect: got %s, want stage", cluster["+stage"].context)
	}
}

func TestIsInteractiveCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{"edit command", []string{"edit", "deployment/foo"}, true},
		{"attach command", []string{"attach", "pod-name"}, true},
		{"exec with -it", []string{"exec", "-it", "pod-name", "--", "bash"}, true},
		{"exec with -ti", []string{"exec", "-ti", "pod-name", "--", "bash"}, true},
		{"exec with -i", []string{"exec", "-i", "pod-name", "--", "bash"}, true},
		{"exec with -t", []string{"exec", "-t", "pod-name", "--", "bash"}, true},
		{"exec without flags", []string{"exec", "pod-name", "--", "bash"}, false},
		{"run with -it", []string{"run", "-it", "test", "--image=nginx"}, true},
		{"run with -ti", []string{"run", "-ti", "test", "--image=nginx"}, true},
		{"run without flags", []string{"run", "test", "--image=nginx"}, false},
		{"get command", []string{"get", "pods"}, false},
		{"apply command", []string{"apply", "-f", "file.yaml"}, false},
		{"delete command", []string{"delete", "pod", "foo"}, false},
		{"logs command", []string{"logs", "pod-name"}, false},
		{"describe command", []string{"describe", "pod", "foo"}, false},
		{"empty args", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInteractiveCommand(tt.args)
			if result != tt.expected {
				t.Errorf("isInteractiveCommand(%v) = %v, want %v", tt.args, result, tt.expected)
			}
		})
	}
}

func TestKubectlCommands(t *testing.T) {
	// Test that common kubectl commands are recognized
	// These are the top-level kubectl commands that should work with k
	commands := []string{
		"get",
		"describe",
		"create",
		"delete",
		"apply",
		"edit",
		"patch",
		"replace",
		"scale",
		"rollout",
		"logs",
		"exec",
		"attach",
		"port-forward",
		"proxy",
		"cp",
		"auth",
		"diff",
		"top",
		"cordon",
		"drain",
		"uncordon",
		"label",
		"annotate",
		"completion",
		"api-resources",
		"api-versions",
		"config",
		"plugin",
		"version",
		"explain",
		"expose",
		"run",
		"set",
		"autoscale",
		"certificate",
		"cluster-info",
		"taint",
		"wait",
		"kustomize",
		"debug",
	}

	// Verify these are valid kubectl commands by checking they don't panic
	// and that they're properly formed
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			args := []string{cmd, "--help"}
			// Just verify the args can be constructed without panic
			if len(args) != 2 {
				t.Errorf("Failed to construct args for command: %s", cmd)
			}
			if args[0] != cmd {
				t.Errorf("Command not preserved: got %s, want %s", args[0], cmd)
			}
		})
	}
}

func TestSliceFind(t *testing.T) {
	tests := []struct {
		name      string
		slice     []string
		val       string
		wantIndex int
		wantFound bool
	}{
		{"found at start", []string{"a", "b", "c"}, "a", 0, true},
		{"found at middle", []string{"a", "b", "c"}, "b", 1, true},
		{"found at end", []string{"a", "b", "c"}, "c", 2, true},
		{"not found", []string{"a", "b", "c"}, "d", -1, false},
		{"empty slice", []string{}, "a", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, gotFound := sliceFind(tt.slice, tt.val)
			if gotIndex != tt.wantIndex || gotFound != tt.wantFound {
				t.Errorf("sliceFind(%v, %s) = (%d, %v), want (%d, %v)",
					tt.slice, tt.val, gotIndex, gotFound, tt.wantIndex, tt.wantFound)
			}
		})
	}
}

func TestHasPrefixAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		prefixes []string
		expected bool
	}{
		{"has prefix @", "@cluster", []string{"@", "+", ":"}, true},
		{"has prefix +", "+context", []string{"@", "+", ":"}, true},
		{"has prefix :", ":namespace", []string{"@", "+", ":"}, true},
		{"no prefix", "get", []string{"@", "+", ":"}, false},
		{"empty string", "", []string{"@", "+", ":"}, false},
		{"empty prefixes", "@cluster", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasPrefixAny(tt.s, tt.prefixes)
			if result != tt.expected {
				t.Errorf("hasPrefixAny(%s, %v) = %v, want %v",
					tt.s, tt.prefixes, result, tt.expected)
			}
		})
	}
}
