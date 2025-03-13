package provider

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	rc := m.Run()
	if rc == 0 && testing.CoverMode() != "" {
		c := testing.Coverage()
		if c < 0.8 {
			fmt.Println("Tests passed but coverage failed at", c)
			rc = -1
		}
	}
	os.Exit(rc)
}
