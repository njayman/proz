package cmd

import "testing"

func TestOpenProjectDetached_EmptyExec(t *testing.T) {
	openProjectDetached(Project{Name: "test", Path: "/nonexistent"}, "")
}
