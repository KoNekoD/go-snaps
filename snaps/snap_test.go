package snaps

import (
	"fmt"
	"os"
	"testing"
)

func TestMatchJsonStruct(t *testing.T) {
	type User struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Email       string `json:"email"`
		Age         int    `json:"age"`
		Preferences struct {
			Theme    string `json:"theme"`
			Language string `json:"language"`
			Design   struct {
				Background string `json:"background"`
				Font       string `json:"font"`
			} `json:"design"`
		} `json:"preferences"`
	}

	u := User{
		ID:    1,
		Name:  "mock-user",
		Email: "mock-user@email.com",
		Age:   29,
		Preferences: struct {
			Theme    string `json:"theme"`
			Language string `json:"language"`
			Design   struct {
				Background string `json:"background"`
				Font       string `json:"font"`
			} `json:"design"`
		}(struct {
			Theme    string
			Language string
			Design   struct {
				Background string
				Font       string
			}
		}{
			Theme:    "dark",
			Language: "en",
			Design: struct {
				Background string
				Font       string
			}{Background: "blue", Font: "serif"},
		}),
	}

	MatchJSON(t, u)
}

func TestMatchJsonStructMultiple(t *testing.T) {
	type User struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Email       string `json:"email"`
		Age         int    `json:"age"`
		Preferences struct {
			Theme    string `json:"theme"`
			Language string `json:"language"`
			Design   struct {
				Background string `json:"background"`
				Font       string `json:"font"`
			} `json:"design"`
		} `json:"preferences"`
	}

	u := User{
		ID:    1,
		Name:  "mock-user",
		Email: "mock-user@email.com",
		Age:   29,
		Preferences: struct {
			Theme    string `json:"theme"`
			Language string `json:"language"`
			Design   struct {
				Background string `json:"background"`
				Font       string `json:"font"`
			} `json:"design"`
		}(struct {
			Theme    string
			Language string
			Design   struct {
				Background string
				Font       string
			}
		}{
			Theme:    "dark",
			Language: "en",
			Design: struct {
				Background string
				Font       string
			}{Background: "blue", Font: "serif"},
		}),
	}

	MatchJSON(t, u)

	u.Age = 30

	MatchJSON(t, u)
}

func TestMatchJsonStructUpdateExisting(t *testing.T) {
	type User struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Email       string `json:"email"`
		Age         int    `json:"age"`
		Preferences struct {
			Theme    string `json:"theme"`
			Language string `json:"language"`
			Design   struct {
				Background string `json:"background"`
				Font       string `json:"font"`
			} `json:"design"`
		} `json:"preferences"`
	}

	u := User{
		ID:    1,
		Name:  "mock-user",
		Email: "mock-user@email.com",
		Age:   29,
		Preferences: struct {
			Theme    string `json:"theme"`
			Language string `json:"language"`
			Design   struct {
				Background string `json:"background"`
				Font       string `json:"font"`
			} `json:"design"`
		}(struct {
			Theme    string
			Language string
			Design   struct {
				Background string
				Font       string
			}
		}{
			Theme:    "dark",
			Language: "en",
			Design: struct {
				Background string
				Font       string
			}{Background: "blue", Font: "serif"},
		}),
	}

	// The second step of this test update snapshot - there are problems on restart if you don't delete the file
	firstSnapPath := fmt.Sprintf(defaultSnap.withTesting(t).constructFilename(defaultSnap.baseCaller(3)), 1)
	if _, err := os.Stat("__snapshots__/" + firstSnapPath); err == nil {
		_ = os.Remove("__snapshots__/" + firstSnapPath)
	}

	MatchJSON(t, u)

	u.Age = 30

	defaultSnap.registry = newSnapRegistry() // 1/2: reset the registry

	updateVAR = "true" // 2/2: activate update snapshots mode

	MatchJSON(t, u)
}
