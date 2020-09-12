package main

import "testing"

func TestParseClusterSingleContext(t *testing.T) {
	cluster, names := ParseCluster("+prod")

	if names[0] != "+prod" {
		t.Errorf("kSpace Name incorrect: got %s, want +prod", names[0])
	}

	if cluster["+prod"].context != "prod" {
		t.Errorf("Context incorrect: got %s, want prod", cluster["+prod"].context)
	}
}

func TestParseClusterMultipleContexts(t *testing.T) {
	cluster, names := ParseCluster("+prod,stage")

	if len(names) != 2 {
		t.Errorf("Incorrect names length: got %d, want 2", len(names))
	}

	if names[0] != "+prod" {
		t.Errorf("kSpace Name incorrect: got %s, want +prod", names[0])
	}

	if cluster["+prod"].context != "prod" {
		t.Errorf("Context incorrect: got %s, want prod", cluster["+prod"].context)
	}

	if names[1] != "+stage" {
		t.Errorf("kSpace Name incorrect: got %s, want +prod", names[0])
	}

	if cluster["+stage"].context != "stage" {
		t.Errorf("Context incorrect: got %s, want stage", cluster["+stage"].context)
	}
}

// TODO find a way to test this because the names slice is not ordered
// func TestParseClusterMultipleContextsWithNamespaces(t *testing.T) {
// 	cluster, names := ParseCluster("+prod,stage:default,kube-system", "")

// 	if len(names) != 4 {
// 		t.Errorf("Incorrect names length: got %d, want 4", len(names))
// 	}

// 	if names[0] != "+prod:default" {
// 		t.Errorf("kSpace Name incorrect: got %s, want +prod:default", names[0])
// 	}

// 	if cluster["+prod:default"].context != "prod" &&
// 		cluster["+prod:default"].namespace != "default" {
// 		t.Errorf("Context or namespace incorrect: got %s and %s, want prod and default",
// 			cluster["+prod:default"].context, cluster["+prod:default"].namespace)
// 	}

// 	if names[2] != "+stage:default" {
// 		t.Errorf("kSpace Name incorrect: got %s, want +stage:default", names[1])
// 	}

// 	if cluster["+stage:kube-system"].context != "stage" {
// 		t.Errorf("Context incorrect: got %s, want stage", cluster["+stage"].context)
// 	}
// }
