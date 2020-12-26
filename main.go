package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	GAME_UNIT_SCV                = 1
	GAME_BUILDING_COMMAND_CENTER = 2
	ELIMINATED                   = "Eliminated"
	VICTORY                      = "Victory"
	GAME_STATUS_FINISHED         = "Finished"
	GAME_STATUS_RUNNING          = "Running"
	GAME_STATUS_PENDING          = "Pending"

	TASK_TYPE_BUILD_SCV = 1

	COST_SCV_MINERALS = 50

	STATUS_IDLE   = 0
	STATUS_MINING = 1
	STATUS_MOVING = 2
)

var lobby *Lobby

func main() {
	lobby = newLobby()
	lobby.games["test"] = newGame()
	lobby.games["test"].status = GAME_STATUS_RUNNING
	lobby.games["test_pending"] = newGame()
	lobby.games["test_pending"].status = GAME_STATUS_PENDING
	go func() {
		for {
			updLobby(lobby)
		}
	}()
	http.Handle("/", new(countHandler))
	log.Fatal(http.ListenAndServe(":8182", nil))
}

func updLobby(l *Lobby) {
	for n, g := range l.games {
		if g.status != GAME_STATUS_RUNNING {
			continue
		}
		now := time.Now()
		passed := now.Sub(g.lastSim)
		if passed < 3*time.Second {
			continue
		}
		if passed > 4*time.Second {
			log.Printf("WARNING: %s game is more than %f seconds late", n, passed.Seconds())
		}
		gameSim(g)
	}
}

func gameSim(g *Game) {
	if len(g.Players) == 0 {
		initGame(g)
	}
	g.objectsMu.Lock()
	killedIDs := make(map[int]bool)
	for i, gob := range g.Objects {
		if gob.Status == STATUS_MINING {
			g.Players[gob.Owner].mu.Lock()
			g.Players[gob.Owner].Minerals += gob.yps
			g.Players[gob.Owner].mu.Unlock()
		}
		if gob.Type == GAME_UNIT_SCV && gob.Status == STATUS_IDLE {
			var targetIDs []int
			for j, pt := range g.Objects {
				if pt.Owner != gob.Owner && pt.Location == gob.Location {
					targetIDs = append(targetIDs, j)
				}
			}
			if len(targetIDs) != 0 {
				targetID := targetIDs[rand.Intn(len(targetIDs))]
				g.Objects[targetID].Hp -= gob.dps
				if g.Objects[targetID].Hp <= 0 {
					killedIDs[targetID] = true
					log.Printf("SCV killed [%d-->%d]", i, targetID)
				}
			}
		}
		if gob.Type == GAME_BUILDING_COMMAND_CENTER && gob.Building.Task != (Task{}) {
			g.Objects[i].Task.Progress += gob.taskSpeed
			if g.Objects[i].Task.Progress >= 100 {
				log.Printf("SCV: good to go sir, %s", gob.Owner)
				g.Objects = append(g.Objects, SCV(gob.Owner, gob.Location))
				g.Objects[i].Task = Task{}
			}
		}
	}
	var nos []GameObject
	buildingsPerPlayer := make(map[string]bool)
	for k, v := range g.Objects {
		if !killedIDs[k] {
			nos = append(nos, v)
			if v.Type == GAME_BUILDING_COMMAND_CENTER {
				buildingsPerPlayer[v.Owner] = true
			}
		}
	}
	g.Objects = nos
	g.objectsMu.Unlock()
	g.lastSim = time.Now()
	for k := range g.Players {
		_, ok := buildingsPerPlayer[k]
		if !ok {
			g.Players[k].mu.Lock()
			g.Players[k].Outcome = ELIMINATED
			g.Players[k].mu.Unlock()
		} else if len(buildingsPerPlayer) == 1 {
			g.status = GAME_STATUS_FINISHED
			g.Players[k].mu.Lock()
			g.Players[k].Outcome = VICTORY
			g.Players[k].mu.Unlock()
		}
	}
}

// Kudos to https://stackoverflow.com/questions/37334119/how-to-delete-an-element-from-a-slice-in-golang
func remove(s []GameObject, i int) []GameObject {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

func initGame(g *Game) {
	g.Players = make(map[string]*Player)
	g.status = GAME_STATUS_RUNNING
	players := []string{"0", "1"}
	for k, pl := range players {
		g.Locations = append(g.Locations, Location{})
		g.Players[pl] = &Player{50, "", sync.Mutex{}}
		for j := 0; j < 5; j++ {
			g.Objects = append(g.Objects, SCV(pl, k))
		}
		g.Objects = append(g.Objects, CommandCenter(pl, k))
	}
	log.Printf("Game started")
}

func CommandCenter(owner string, location int) GameObject {
	return GameObject{
		Owner:    owner,
		Location: location,
		Hp:       1500,
		HpMax:    1500,
		Type:     GAME_BUILDING_COMMAND_CENTER,
		Building: Building{
			taskSpeed: 20,
		},
	}
}

func SCV(owner string, location int) GameObject {
	return GameObject{
		Owner:    owner,
		Location: location,
		Hp:       60,
		HpMax:    60,
		Type:     GAME_UNIT_SCV,
		Unit: Unit{
			dps:   8,
			speed: 4,
			yps:   1,
		},
	}
}

type Location struct{}

type Player struct {
	Minerals int
	Outcome  string
	mu       sync.Mutex
}

type Game struct {
	Players   map[string]*Player
	Locations []Location
	Objects   []GameObject
	objectsMu sync.Mutex
	lastSim   time.Time
	status    string
}

func newGame() *Game {
	g := &Game{}
	g.Players = make(map[string]*Player)
	g.lastSim = time.Now()
	return g
}

type Lobby struct {
	games map[string]*Game
}

func newLobby() *Lobby {
	l := &Lobby{}
	l.games = make(map[string]*Game)
	return l
}

func (g Game) String() string {
	b, err := json.Marshal(g)
	if err != nil {
		log.Printf("ERROR json.Marshal %v", err)
		return ""
	}
	return string(b)
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

func (g Game) Export(player string) string {
	eg := newGame()
	eg.Players[player] = g.Players[player]
	for _, v := range g.Locations {
		eg.Locations = append(eg.Locations, v)
	}
	visLocIds := make(map[int]bool)
	for _, v := range g.Objects {
		if v.Owner == player {
			visLocIds[v.Location] = true
		}
	}
	for _, v := range g.Objects {
		if _, ok := visLocIds[v.Location]; ok {
			eg.Objects = append(eg.Objects, v)
		}
	}

	b, err := json.Marshal(eg)
	if err != nil {
		log.Printf("ERROR json.Marshal for the game %v %v", g, err)
		return ""
	}
	return string(b)
}

type GameObject struct {
	Owner    string
	Location int
	Hp       int
	HpMax    int
	Type     int
	Building
	Unit
}

type Building struct {
	Task      Task
	taskSpeed int
	Queue     []int // An array of task types
}

type Task struct {
	Type     int
	Progress int
}

type Unit struct {
	dps    int
	speed  int
	Status int
	yps    int
}

type countHandler struct{}

func getGetIntParam(values url.Values, name string) (int, error) {
	if v, ok := values[name]; ok {
		if len(v) != 1 {
			return 0, fmt.Errorf("More than one GET parameter %s", name)
		}
		if iv, err := strconv.Atoi(v[0]); err == nil {
			return iv, nil
		} else {
			return 0, fmt.Errorf("GET parameter %s is not a number: %s", name, v)
		}
	} else {
		return 0, fmt.Errorf("No GET parameter %s", name)
	}
}

func getGetStrParam(values url.Values, name string) (string, error) {
	if v, ok := values[name]; ok {
		if len(v) != 1 {
			return "", fmt.Errorf("More than one GET parameter %s", name)
		}
		return v[0], nil
	} else {
		return "", fmt.Errorf("No GET parameter %s", name)
	}
}

func getLocationID(g *Game, values url.Values, name string) (int, error) {
	locID, err := getGetIntParam(values, name)
	if err != nil {
		return locID, err
	}
	if locID >= len(g.Locations) {
		return locID, fmt.Errorf("No such location %d", locID)
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

func (h *countHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request received from %s, url: %s", r.RemoteAddr, r.URL)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	q := r.URL.Query()

	player, err := getGetStrParam(q, "player")
	if err != nil {
		fmt.Fprintf(w, "%s", httpError(err))
		return
	}

	g := getPlayerGame(lobby, player)
	if g == nil {
		fmt.Fprintf(w, "%s", exportPendingGames(lobby))
		return
	}
	if g.status == GAME_STATUS_FINISHED || !checkGetParamExists(q, "location_id") {
		fmt.Fprintf(w, "%s", g.Export(player))
		return
	}

	locID, err := getLocationID(g, q, "location_id")
	if err != nil {
		fmt.Fprintf(w, "%s", httpError(err))
		return
	}

	if checkGetParamExists(q, "build_scv") {
		log.Printf("Player: %s is building a SCV", player)
		err = buildSCV(g, player, locID)
		if err != nil {
			fmt.Fprintf(w, "%s", httpError(err))
			return
		}
		fmt.Fprintf(w, "%s", `{"status":"ok"}`)
		return
	}

	if checkGetParamExists(q, "scv_to_work") {
		log.Printf("Player: %s is sending SCV to work", player)
		err = statusSCV(g, player, locID, STATUS_IDLE, STATUS_MINING)
		if err != nil {
			fmt.Fprintf(w, "%s", httpError(err))
			return
		}
		fmt.Fprintf(w, "%s", `{"status":"ok"}`)
		return
	}

	if checkGetParamExists(q, "idle_scv") {
		log.Printf("Player: %s is sending SCV to idle", player)
		err = statusSCV(g, player, locID, STATUS_MINING, STATUS_IDLE)
		if err != nil {
			fmt.Fprintf(w, "%s", httpError(err))
			return
		}
		fmt.Fprintf(w, "%s", `{"status":"ok"}`)
		return
	}

	if checkGetParamExists(q, "destination_id") {
		destID, err := getLocationID(g, q, "destination_id")
		if err != nil {
			fmt.Fprintf(w, "%s", httpError(err))
			return
		}

		log.Printf("Player: %s is sending SCV [%d-->%d]", player, locID, destID)
		err = sendSCV(g, player, locID, destID)
		if err != nil {
			fmt.Fprintf(w, "%s", httpError(err))
			return
		}
		fmt.Fprintf(w, "%s", `{"status":"ok"}`)
		return
	}

	fmt.Fprintf(w, "%s", `{"error":{"message":"Not supported action"}}`)
}

func sendSCV(g *Game, player string, locID int, destID int) error {
	g.objectsMu.Lock()
	defer g.objectsMu.Unlock()
	for i, gob := range g.Objects {
		if gob.Type == GAME_UNIT_SCV && gob.Owner == player && gob.Location == locID && gob.Status == STATUS_IDLE {
			g.Objects[i].Location = destID
			return nil
		}
	}
	return fmt.Errorf("Couldn't find any IDLE SCVs at location %d for player %s", locID, player)
}

func statusSCV(g *Game, player string, locID int, status_from int, status_to int) error {
	g.objectsMu.Lock()
	defer g.objectsMu.Unlock()
	for i, gob := range g.Objects {
		if gob.Type == GAME_UNIT_SCV && gob.Owner == player && gob.Location == locID && gob.Status == status_from {
			g.Objects[i].Status = status_to
			return nil
		}
	}
	if status_from == STATUS_MINING {
		return fmt.Errorf("Couldn't find any MINING SCVs at location %d for player %s", locID, player)
	}
	return fmt.Errorf("Couldn't find any IDLE SCVs at location %d for player %s", locID, player)
}

func buildSCV(g *Game, player string, locID int) error {
	g.objectsMu.Lock()
	defer g.objectsMu.Unlock()

	ccFound := false
	var ccID int
	for i, gob := range g.Objects {
		if gob.Type == GAME_BUILDING_COMMAND_CENTER && gob.Location == locID && gob.Owner == player {
			ccFound = true
			ccID = i
		}
	}
	if !ccFound {
		return fmt.Errorf("No command center at location %d", locID)
	}
	if g.Objects[ccID].Building.Task != (Task{}) {
		return fmt.Errorf("The command center is busy, sorry")
	}
	pl := g.Players[player]
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if pl.Minerals < COST_SCV_MINERALS {
		return fmt.Errorf("Not enogh minerals, need %d, have %d", COST_SCV_MINERALS, pl.Minerals)
	}
	pl.Minerals -= COST_SCV_MINERALS
	g.Objects[ccID].Building.Task = Task{Type: TASK_TYPE_BUILD_SCV}
	return nil
}

func httpError(err error) string {
	return fmt.Sprintf(`{"error":{"message":"%s"}}`, strings.Replace(err.Error(), `"`, `\"`, -1))
}
