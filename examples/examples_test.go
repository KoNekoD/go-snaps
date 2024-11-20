package examples

import (
	"flag"
	"fmt"
	"github.com/gkampitakis/go-snaps/snaps"
	"testing"
)

// You can use -update flag to control if you want to update the snapshots
// go test ./... -v -update
var updateSnapshot = flag.Bool("update", false, "update snapshots flag")

func TestUpdateWithFlag(t *testing.T) {
	config := snaps.WithConfig(snaps.Update(*updateSnapshot))

	inputs := []string{
		"lore ipsum dolor sit amet",
		"consectetur adipiscing elit",
		"sed do eiusmod tempor incididunt ut labore et dolore magna aliqua",
		"Ut enim ad minim veniam, quis nostrud laboris nisi ut aliquip ex ea commodo consequat.",
	}

	for i, input := range inputs {
		t.Run(fmt.Sprintf("test - %d", i), func(t *testing.T) {
			config.MatchSnapshot(t, input)
		})
	}
}
