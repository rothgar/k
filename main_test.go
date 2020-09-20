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
