package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestCommands(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"interfaces"}, {"config", "validate"}, {"help"}} {
		var out, errOut bytes.Buffer
		if err := Run(context.Background(), args, &out, &errOut, func(string) string { return "" }); err != nil {
			t.Fatal(err)
		}
		if out.Len() == 0 {
			t.Fatal("no output")
		}
		if args[0] == "interfaces" && !json.Valid(out.Bytes()) {
			t.Fatal("invalid interfaces JSON")
		}
	}
}
func TestInvalidCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"version", "extra"}, {"interfaces", "extra"}, {"serve", "--bad"}, {"serve", "extra"}, {"config"}, {"config", "write"}, {"config", "validate", "--config", "missing.yaml"}} {
		var out, errOut bytes.Buffer
		if err := Run(context.Background(), args, &out, &errOut, nil); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
