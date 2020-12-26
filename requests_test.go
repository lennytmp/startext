package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJustPlayer(t *testing.T) {
	api := apiHandler{}
	req, err := http.NewRequest("GET", "/?player=0", nil)
	if err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		name  string
		games map[string]*Game
		want  string
	}{
		{
			name:  "zero pending games",
			games: make(map[string]*Game),
			want:  "{}",
		},
		{
			name: "1 pending games with other players",
			games: map[string]*Game{
				"test": &Game{Players: map[string]*Player{"lenny": &Player{}}, status: GAME_STATUS_PENDING},
			},
			want: `{"test":{"Players":{"lenny":{"Minerals":0,"Outcome":""}},"Locations":null,"Objects":null}}`,
		},
	}
	for _, tc := range testCases {
		lobby = newLobby()
		lobby.games = tc.games
		rr := httptest.NewRecorder()
		api.ServeHTTP(rr, req)
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("%s: wrong status code: got %v want %v", tc.name, status, http.StatusOK)
		}
		if rr.Body.String() != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, rr.Body.String(), tc.want)
		}
	}
}
