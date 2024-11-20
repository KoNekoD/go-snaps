package snaps

import "testing"

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
