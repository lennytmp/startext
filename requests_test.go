package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartPending(t *testing.T) {
	api := apiHandler{}
	lobby = newLobby()
	lobby.games["test"] = &Game{
		Players: map[string]*Player{"0": &Player{}, "1": &Player{}},
		status:  GAME_STATUS_PENDING,
	}
	{ // Player0 is ready
		req, err := http.NewRequest("GET", "/?player=0&ready", nil)
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		api.ServeHTTP(rr, req)
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("wrong status code: got %v want %v", status, http.StatusOK)
		}
		wantResp := `{"status":"ok"}`
		if rr.Body.String() != wantResp {
			t.Errorf("got %v want %v", rr.Body.String(), wantResp)
		}
	}

	{
		req, err := http.NewRequest("GET", "/?player=1&ready", nil)
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		api.ServeHTTP(rr, req)
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("wrong status code: got %v want %v", status, http.StatusOK)
		}
		wantResp := `{"status":"ok"}`
		if rr.Body.String() != wantResp {
			t.Errorf("got %v want %v", rr.Body.String(), wantResp)
		}
	}

	g := lobby.games["test"]
	if g.status != GAME_STATUS_RUNNING {
		t.Errorf("wanted status running, got %s", g.status)
	}
	for n, p := range g.Players {
		if p.Minerals != 50 {
			t.Errorf("expected 50 minerals for player %s, got %d", n, p.Minerals)
		}
		if p.Outcome != "" {
			t.Errorf("Game has just started, but there is already an outcome %s for player %s", p.Outcome, n)
		}
	}
}

func TestQuit(t *testing.T) {
	api := apiHandler{}
	req, err := http.NewRequest("GET", "/?player=0&quit", nil)
	if err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		name     string
		game     *Game
		wantResp string
		wantGame string
	}{
		{
			game: &Game{
				name: "Pending game 2 players",
				Players: map[string]*Player{
					"0": &Player{},
					"2": &Player{},
				},
				status: GAME_STATUS_PENDING,
			},
			wantResp: `{"status":"ok"}`,
			wantGame: `{"Players":{"2":{"Minerals":0,"Outcome":"","Ready":false}},"Locations":null,"Objects":null}`,
		},
	}
	for _, tc := range testCases {
		lobby = newLobby()
		lobby.games["test"] = tc.game
		rr := httptest.NewRecorder()
		api.ServeHTTP(rr, req)
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("%s: wrong status code: got %v want %v", tc.name, status, http.StatusOK)
		}
		if rr.Body.String() != tc.wantResp {
			t.Errorf("%s: got %v want %v", tc.name, rr.Body.String(), tc.wantResp)
		}
		gotGame := lobby.games["test"].exportAll()
		if gotGame != tc.wantGame {
			t.Errorf("%s: got %v want %v", tc.name, gotGame, tc.wantGame)
		}
	}
}

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
			want: `{"test":{"Players":{"lenny":{"Minerals":0,"Outcome":"","Ready":false}},"Locations":null,"Objects":null}}`,
		},
		{
			name: "1 running games with other players",
			games: map[string]*Game{
				"test": &Game{Players: map[string]*Player{"lenny": &Player{}}, status: GAME_STATUS_RUNNING},
			},
			want: `{}`,
		},
		{
			name: "1 finished games with other players",
			games: map[string]*Game{
				"test": &Game{Players: map[string]*Player{"lenny": &Player{}}, status: GAME_STATUS_FINISHED},
			},
			want: `{}`,
		},
		{
			name: "1 my pending game",
			games: map[string]*Game{
				"test": &Game{
					Players: map[string]*Player{
						"0": &Player{},
						"2": &Player{},
					},
					status: GAME_STATUS_PENDING,
				},
			},
			want: `{"Players":{"0":{"Minerals":0,"Outcome":"","Ready":false},"2":{"Minerals":0,"Outcome":"","Ready":false}},"Locations":null,"Objects":null}`,
		},
		{
			name: "1 my running game",
			games: map[string]*Game{
				"test": &Game{
					Players: map[string]*Player{
						"0": &Player{},
						"2": &Player{},
					},
					status: GAME_STATUS_RUNNING},
			},
			want: `{"Players":{"0":{"Minerals":0,"Outcome":"","Ready":false}},"Locations":null,"Objects":null}`,
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
