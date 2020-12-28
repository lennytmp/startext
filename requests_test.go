package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeMakeBotRequest(url string) ([]byte, error) {
	status, body, err := makeTestRequest(url)
	var res []byte
	if err != nil {
		return res, fmt.Errorf("sending request failed %v", err)
	}
	if status != http.StatusOK {
		return res, fmt.Errorf("bad status code: %v", status)
	}
	return []byte(body), nil
}

func TestPlayWithBot(t *testing.T) {
	lobby = newLobby()
	botTriggerQueue = make(chan triggerRequest, 50)
	g := &Game{
		Players: map[string]*Player{"0": &Player{}},
		status:  GAME_STATUS_PENDING,
	}
	lobby.games["test"] = g
	{
		status, body, err := makeTestRequest("/?player=0&add_bot")
		if err != nil {
			t.Fatal(err)
		}
		if status != http.StatusOK {
			t.Errorf("wrong status code: got %v want %v", status, http.StatusOK)
		}
		wantResp := `"status":"ok"`
		if !strings.Contains(body, wantResp) {
			t.Errorf("got %v wanted %v as a substring", body, wantResp)
		}
	}
	if l := len(lobby.games["test"].Players); l != 2 {
		t.Fatalf("expected 2 players, go %d", l)
	}
	origBotUpdDelay := BOT_UPDATE_DELAY
	BOT_UPDATE_DELAY = -1
	makeBotRequestOverridable = fakeMakeBotRequest
	defer func() {
		BOT_UPDATE_DELAY = origBotUpdDelay
		makeBotRequestOverridable = makeBotRequest
	}()
	{
		status, body, err := makeTestRequest("?player=0&ready")
		if err != nil {
			t.Fatal(err)
		}
		if status != http.StatusOK {
			t.Errorf("wrong status code: got %v want %v", status, http.StatusOK)
		}
		wantResp := `"status":"ok"`
		if !strings.Contains(body, wantResp) {
			t.Errorf("got %v wanted %v as a substring", body, wantResp)
		}
	}
	if g.status != GAME_STATUS_RUNNING {
		t.Errorf("wanted status running, got %s", g.status)
	}
	processBotQueue()
}

func TestStartPending(t *testing.T) {
	lobby = newLobby()
	lobby.games["test"] = &Game{
		Players: map[string]*Player{"0": &Player{}, "1": &Player{}},
		status:  GAME_STATUS_PENDING,
	}
	for _, rURL := range []string{"/?player=0&ready", "?player=1&ready"} {
		status, body, err := makeTestRequest(rURL)
		if err != nil {
			t.Fatal(err)
		}
		if status != http.StatusOK {
			t.Errorf("wrong status code: got %v want %v", status, http.StatusOK)
		}
		wantResp := `"status":"ok"`
		if !strings.Contains(body, wantResp) {
			t.Errorf("got %v wanted %v as a substring", body, wantResp)
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

func makeTestRequest(url string) (int, string, error) {
	api := apiHandler{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, "", nil
	}
	rr := httptest.NewRecorder()
	api.ServeHTTP(rr, req)
	return rr.Code, rr.Body.String(), nil
}

func TestQuit(t *testing.T) {
	lobby = newLobby()
	lobby.games["test"] = &Game{
		name: "Pending game 2 players",
		Players: map[string]*Player{
			"0": &Player{},
			"2": &Player{},
		},
		status: GAME_STATUS_PENDING,
	}
	status, body, err := makeTestRequest("/?player=0&quit")
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK {
		t.Errorf("wrong status code: got %v want %v", status, http.StatusOK)
	}
	wantResp := `"status":"ok"`
	if !strings.Contains(body, wantResp) {
		t.Errorf("got %v wanted %v as a substring", body, wantResp)
	}
	wantGame := `{"Players":{"2":{"Minerals":0,"Outcome":"","Ready":false}},"Locations":null,"Objects":null}`
	gotGame := lobby.games["test"].exportAll()
	if gotGame != wantGame {
		t.Errorf("got game %v want %v", gotGame, wantGame)
	}
}

func TestJustPlayer(t *testing.T) {
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
		status, body, err := makeTestRequest("?player=0")
		if err != nil {
			t.Fatal(err)
		}
		if status != http.StatusOK {
			t.Errorf("%s: wrong status code: got %v want %v", tc.name, status, http.StatusOK)
		}
		if body != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, body, tc.want)
		}
	}
}
