package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var lobby *Lobby

func main() {
	lobby = newLobby()
	botTriggerQueue = make(chan triggerRequest)
	go func() {
		for {
			lobby.mu.Lock()
			updLobby(lobby)
			lobby.mu.Unlock()
		}
	}()
	http.Handle("/", new(apiHandler))
	log.Fatal(http.ListenAndServe(":8182", nil))
}

func updLobby(l *Lobby) {
	for n, g := range l.games {
		func() {
			g.mu.Lock()
			defer g.mu.Unlock()

			if g.status != GAME_STATUS_RUNNING {
				return
			}
			now := time.Now()
			passed := now.Sub(g.lastSim)
			if passed < 3*time.Second {
				return
			}
			if passed > 4*time.Second {
				log.Printf("WARNING: %s game is more than %f seconds late", n, passed.Seconds())
			}
			gameSim(g)
		}()
	}
}

type Lobby struct {
	games map[string]*Game
	mu    sync.Mutex
}

func newLobby() *Lobby {
	l := &Lobby{}
	l.games = make(map[string]*Game)
	return l
}

func exportPendingGames(l *Lobby) string {
	pending := make(map[string]*Game)
	for gn, g := range l.games {
		if g.status == GAME_STATUS_PENDING {
			pending[gn] = g
		}
	}
	b, err := json.Marshal(pending)
	if err != nil {
		log.Printf("ERROR json.Marshal for pending games %v %v", pending, err)
		return ""
	}
	return string(b)
}

type apiHandler struct{}

func getGetIntParam(values url.Values, name string) (int, error) {
	if v, ok := values[name]; ok {
		if len(v) != 1 {
			return 0, fmt.Errorf("more than one GET parameter %s", name)
		}
		if iv, err := strconv.Atoi(v[0]); err == nil {
			return iv, nil
		} else {
			return 0, fmt.Errorf("GET parameter %s is not a number: %s", name, v)
		}
	} else {
		return 0, fmt.Errorf("no GET parameter %s", name)
	}
}

func getGetStrParam(values url.Values, name string) (string, error) {
	if v, ok := values[name]; ok {
		if len(v) != 1 {
			return "", fmt.Errorf("more than one GET parameter %s", name)
		}
		return v[0], nil
	} else {
		return "", fmt.Errorf("no GET parameter %s", name)
	}
}

func getLocationID(g *Game, values url.Values, name string) (int, error) {
	locID, err := getGetIntParam(values, name)
	if err != nil {
		return locID, err
	}
	if locID >= len(g.Locations) {
		return locID, fmt.Errorf("no such location %d", locID)
	}
	return locID, nil
}

func checkGetParamExists(values url.Values, name string) bool {
	_, ok := values[name]
	return ok
}

func getPlayerGame(l *Lobby, player string) *Game {
	for _, g := range l.games {
		if _, ok := g.Players[player]; ok {
			return g
		}
	}
	return nil
}

func joinGame(l *Lobby, player string, gameName string) error {
	g, ok := l.games[gameName]
	if !ok {
		l.games[gameName] = newGame(gameName)
		g = l.games[gameName]
	}
	g.Players[player] = &Player{}
	return nil
}

func quitGame(l *Lobby, g *Game, player string) {
	if len(g.Players) == 1 {
		delete(l.games, g.name)
		return
	}
	if g.status == GAME_STATUS_PENDING {
		delete(g.Players, player)
		return
	}
	nos := []GameObject{}
	for _, o := range g.Objects {
		if o.Owner != player {
			nos = append(nos, o)
		}
	}
	g.Objects = nos
	delete(g.Players, player)
}

func getPlayerName(values url.Values) (string, error) {
	player, err := getGetStrParam(values, "player")
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(player, "bot") {
		return "", fmt.Errorf("player name can't start with prefix bot, sorry")
	}
	return player, nil
}

func handleNoGame(w *http.ResponseWriter, values url.Values, player string) {
	if !checkGetParamExists(values, "game") {
		fmt.Fprintf(*w, "%s", exportPendingGames(lobby))
		return
	}
	gameName, err := getGetStrParam(values, "game")
	if err != nil {
		httpGiveErr(w, err)
		return
	}
	httpGiveErr(w, joinGame(lobby, player, gameName))
}

func handlePendingGame(w *http.ResponseWriter, values url.Values, player string, g *Game) {
	if checkGetParamExists(values, "add_bot") {
		p := Player{}
		p.bot = true
		g.Players["bot"+strconv.Itoa(len(g.Players))] = &p
		httpGiveStatus(w, nil, fmt.Sprintf("A bot was added to the game %s.", g.name))
		return
	}
	if checkGetParamExists(values, "ready") {
		func() {
			g.Players[player].Ready = true
			if len(g.Players) == 1 {
				return
			}
			for _, p := range g.Players {
				if !p.Ready {
					return
				}
			}
			initGame(g)
		}()
		httpGiveStatus(w, nil, "The ready status is set.")
		return
	}
	fmt.Fprintf(*w, "%s", g.Export(player))
}

func handleRunningGame(w *http.ResponseWriter, values url.Values, player string, g *Game) {
	if !checkGetParamExists(values, "location_id") {
		fmt.Fprintf(*w, "%s", g.Export(player))
		return
	}
	locID, err := getLocationID(g, values, "location_id")
	if err != nil {
		httpGiveErr(w, err)
		return
	}

	if checkGetParamExists(values, "build_scv") {
		log.Printf("Player: %s is building a SCV", player)
		err = buildSCV(g, player, locID)
		httpGiveErr(w, err)
		return
	}

	if checkGetParamExists(values, "scv_to_work") {
		log.Printf("Player: %s is sending SCV to work", player)
		err = statusSCV(g, player, locID, STATUS_IDLE, STATUS_MINING)
		httpGiveErr(w, err)
		return
	}

	if checkGetParamExists(values, "idle_scv") {
		log.Printf("Player: %s is sending SCV to idle", player)
		err = statusSCV(g, player, locID, STATUS_MINING, STATUS_IDLE)
		httpGiveErr(w, err)
		return
	}

	if checkGetParamExists(values, "destination_id") {
		destID, err := getLocationID(g, values, "destination_id")
		if err != nil {
			httpGiveErr(w, err)
			return
		}

		log.Printf("Player: %s is sending SCV [%d-->%d]", player, locID, destID)
		err = sendSCV(g, player, locID, destID)
		httpGiveErr(w, err)
		return
	}
	fmt.Fprintf(*w, "%s", g.Export(player))
}

func handleGame(w *http.ResponseWriter, values url.Values, player string, g *Game) {
	if checkGetParamExists(values, "quit") {
		quitGame(lobby, g, player)
		httpGiveStatus(w, nil, "You succesfully quit the game.")
		return
	}

	if g.status == GAME_STATUS_PENDING {
		handlePendingGame(w, values, player, g)
		return
	}

	if g.status == GAME_STATUS_FINISHED {
		fmt.Fprintf(*w, "%s", g.Export(player))
		return
	}
	handleRunningGame(w, values, player, g)
}

func (h *apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request received from %s, url: %s", r.RemoteAddr, r.URL)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	q := r.URL.Query()

	player, err := getPlayerName(q)
	if err != nil {
		httpGiveErr(&w, err)
		return
	}

	lobby.mu.Lock()
	defer lobby.mu.Unlock()
	g := getPlayerGame(lobby, player)
	if g == nil {
		handleNoGame(&w, q, player)
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	handleGame(&w, q, player, g)
}

func httpGiveErr(w *http.ResponseWriter, err error) {
	httpGiveStatus(w, err, "")
}

func httpGiveStatus(w *http.ResponseWriter, err error, success string) {
	if err == nil {
		if success != "" {
			fmt.Fprintf(*w, "%s", fmt.Sprintf(`{"data":{"status":"ok"}}`))
			return
		}
		fmt.Fprintf(*w, "%s", fmt.Sprintf(`{"data":{"status":"ok",message:"%s"}}`, success))
		return
	}
	fmt.Fprintf(*w, "%s", fmt.Sprintf(`{"error":{"message":"%s"}}`, strings.Replace(err.Error(), `"`, `\"`, -1)))
}
