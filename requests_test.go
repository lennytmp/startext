package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	for _, gob := range g.Objects {
		if gob.Owner == "0" {
			continue
		}
		if gob.Unit.Type == UNIT_SCV && gob.Unit.Status == UNIT_STATUS_IDLE {
			t.Errorf("Expected the bot to send all SCVs to mine minerals, found idle instead %v", gob)
		}
		if gob.Building.Type == BUILDING_COMMAND_CENTER && gob.Task == (Task{}) {
			t.Errorf("Expected the bot to start producing SCV, but nothing is queued %v", gob)
		}
	}
}

func TestBuildBarracks(t *testing.T) {
	lobby = newLobby()
	g := &Game{
		Players: map[string]*Player{"0": &Player{}, "1": &Player{}},
		status:  GAME_STATUS_PENDING,
	}
	initGame(g)
	testOwner := "0"
	g.Players[testOwner].Minerals = 150
	homeID := 0
	for _, gob := range g.Objects {
		if gob.Owner == testOwner && gob.Building.Type == BUILDING_COMMAND_CENTER {
			homeID = gob.Location
		}
	}
	lobby.games["test"] = g
	status, body, err := makeTestRequest(fmt.Sprintf("/?player=%s&location_id=%d&build=barracks", testOwner, homeID))
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
	found := false
	testBuildTime := 300 * time.Millisecond
	for i, gob := range g.Objects {
		if gob.Owner == testOwner && gob.Building.Type == BUILDING_BARRACKS {
			found = true
			g.Objects[i].TimeToBuild = testBuildTime.Milliseconds()
			g.Objects[i].LeftToBuild = testBuildTime.Milliseconds()
			break
		}
	}
	if !found {
		t.Errorf("barracks not found in game objects %v", g.Export("0"))
	}
	if g.Players[testOwner].Minerals != 0 {
		t.Errorf("expected 0 balance for player 0, but found %d", g.Players[testOwner].Minerals)
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*testBuildTime)
	defer cancel()
	barracksID := 0
	for {
		gameSim(g)
		done := false
		for k, gob := range g.Objects {
			if gob.Owner == testOwner && gob.Building.Type == BUILDING_BARRACKS && gob.Hp == gob.HpMax {
				done = true
				barracksID = k
				break
			}
		}
		if done {
			break
		}
		if ctx.Err() != nil {
			t.Errorf("barracks were not built in time: %s", g.Export(testOwner))
			break
		}
	}
	elapsed := time.Now().Sub(start)
	highB := (testBuildTime + 100*time.Millisecond).Milliseconds()
	if elapsed.Milliseconds() < testBuildTime.Milliseconds() || elapsed.Milliseconds() > highB {
		t.Errorf("expected construction to take %dms but it took %dms", testBuildTime.Milliseconds(), elapsed.Milliseconds())
	}
	if st := g.Objects[barracksID].Building.Status; st != BUILDING_STATUS_IDLE {
		t.Errorf("expected idle status for barracks, got %s", st)
	}
	if g.Objects[barracksID].Hp < g.Objects[barracksID].HpMax {
		t.Errorf("expected hp to be hpMax but got %d[%d]", g.Objects[barracksID].Hp, g.Objects[barracksID].HpMax)
	}
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
