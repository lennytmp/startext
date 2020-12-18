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
	GAME_UNIT_SCV                = 0
	GAME_BUILDING_COMMAND_CENTER = 0
	TASK_TYPE_BUILD_SCV          = 0
	COST_SCV_MINERALS            = 50

	STATUS_IDLE   = 0
	STATUS_MINING = 1
	STATUS_MOVING = 2
)

var testGame Game

func main() {
	testGame = Game{}
	go func() {
		for {
			start := time.Now()
			gameSim(&testGame)
			elapsed := time.Now().Sub(start)
			time.Sleep(3*time.Second - elapsed)
			fmt.Printf("[%s]: Elapsed: %s\n", time.Now(), elapsed)
		}
	}()
	http.Handle("/", new(countHandler))
	log.Fatal(http.ListenAndServe(":8182", nil))
}

func gameSim(g *Game) {
	if len(g.Players) == 0 {
		initGame(g)
	}
	g.objectsMu.Lock()
	var killedIDs []int
	for i, gob := range testGame.Objects {
		if gob.Status == STATUS_MINING {
			g.Players[gob.Owner].mu.Lock()
			g.Players[gob.Owner].Minerals += gob.yps
			g.Players[gob.Owner].mu.Unlock()
		}
		if gob.Status == STATUS_IDLE {
			var targetIDs []int
			for j, pt := range testGame.Objects {
				if pt.Owner != gob.Owner && pt.Location == gob.Location {
					targetIDs = append(targetIDs, j)
				}
			}
			if len(targetIDs) != 0 {
				targetID := targetIDs[rand.Intn(len(targetIDs))]
				g.Objects[targetID].Hp -= gob.dps
				if g.Objects[targetID].Hp <= 0 {
					killedIDs = append(killedIDs, targetID)
					fmt.Printf("[%s]: SCV killed [%d-->%d]\n", time.Now(), i, targetID)
				}
			}
		}
	}
	for i := range killedIDs {
		g.Objects = remove(g.Objects, killedIDs[i])
	}
	g.objectsMu.Unlock()
}

// Kudos to https://stackoverflow.com/questions/37334119/how-to-delete-an-element-from-a-slice-in-golang
func remove(s []GameObject, i int) []GameObject {
	s[len(s)-1], s[i] = s[i], s[len(s)-1]
	return s[:len(s)-1]
}

func initGame(g *Game) {
	for i := 0; i < 2; i++ {
		g.Locations = append(g.Locations, Location{})
		g.Players = append(g.Players, Player{50, sync.Mutex{}})
		for j := 0; j < 5; j++ {
			g.Objects = append(g.Objects, SCV(i, i))
		}
		g.Objects = append(g.Objects, CommandCenter(i, i))
	}
	fmt.Printf("[%s]: Game started\n", time.Now())
}

func CommandCenter(owner int, location int) GameObject {
	return GameObject{
		Owner:    owner,
		Location: location,
		Hp:       1500,
		HpMax:    1500,
		Type:     GAME_BUILDING_COMMAND_CENTER,
		Building: Building{
			taskSpeed: 1,
		},
	}
}

func SCV(owner int, location int) GameObject {
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
	mu       sync.Mutex
}

type Game struct {
	Players   []Player
	Locations []Location
	Objects   []GameObject
	objectsMu sync.Mutex
}

func (g Game) String() string {
	b, err := json.Marshal(g)
	if err != nil {
		fmt.Printf("[%s]:ERROR json.Marshal %v %v\n", time.Now(), g, err)
		return ""
	}
	return string(b)
}

func (g Game) Export(playerID int) string {
	eg := Game{}
	eg.Players = append(eg.Players, g.Players[playerID])
	for _, v := range g.Locations {
		eg.Locations = append(eg.Locations, v)
	}
	visLocIds := make(map[int]bool)
	for _, v := range g.Objects {
		if v.Owner == playerID {
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
		fmt.Printf("[%s]:ERROR json.Marshal %v %v\n", time.Now(), g, err)
		return ""
	}
	return string(b)
}

type GameObject struct {
	Owner    int
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

func getPlayerID(values url.Values) (int, error) {
	playerID, err := getGetIntParam(values, "player_id")
	if err != nil {
		return playerID, err
	}
	if playerID >= len(testGame.Players) {
		return playerID, fmt.Errorf("No such player %d", playerID)
	}
	return playerID, nil
}

func getLocationID(values url.Values, name string) (int, error) {
	locID, err := getGetIntParam(values, name)
	if err != nil {
		return locID, err
	}
	if locID >= len(testGame.Locations) {
		return locID, fmt.Errorf("No such location %d", locID)
	}
	return locID, nil
}

func checkGetParamExists(values url.Values, name string) bool {
	_, ok := values[name]
	return ok
}

func (h *countHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[%s]: Request received from %s, url: %s\n", time.Now(), r.RemoteAddr, r.URL)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	q := r.URL.Query()
	// If no player is given - let them observe all.
	if !checkGetParamExists(q, "player_id") {
		fmt.Fprintf(w, "%s", testGame.String())
		return
	}

	playerID, err := getPlayerID(q)
	if err != nil {
		fmt.Fprintf(w, "%s", httpError(err))
		return
	}
	if !checkGetParamExists(q, "location_id") {
		fmt.Fprintf(w, "%s", testGame.Export(playerID))
	}

	locID, err := getLocationID(q, "location_id")
	if err != nil {
		fmt.Fprintf(w, "%s", httpError(err))
		return
	}

	if checkGetParamExists(q, "build_scv") {
		fmt.Printf("[%s]: PlayerId: %d is building a SCV\n", time.Now(), playerID)
		err = buildSCV(playerID, locID)
		if err != nil {
			fmt.Fprintf(w, "%s", httpError(err))
			return
		}
		fmt.Fprintf(w, "%s", `{"status":"ok"}`)
		return
	}

	if checkGetParamExists(q, "scv_to_work") {
		fmt.Printf("[%s]: PlayerId: %d is sending SCV to work\n", time.Now(), playerID)
		err = statusSCV(playerID, locID, STATUS_IDLE, STATUS_MINING)
		if err != nil {
			fmt.Fprintf(w, "%s", httpError(err))
			return
		}
		fmt.Fprintf(w, "%s", `{"status":"ok"}`)
		return
	}

	if checkGetParamExists(q, "idle_scv") {
		fmt.Printf("[%s]: PlayerId: %d is sending SCV to idle\n", time.Now(), playerID)
		err = statusSCV(playerID, locID, STATUS_MINING, STATUS_IDLE)
		if err != nil {
			fmt.Fprintf(w, "%s", httpError(err))
			return
		}
		fmt.Fprintf(w, "%s", `{"status":"ok"}`)
		return
	}

	if checkGetParamExists(q, "destination_id") {
		destID, err := getLocationID(q, "destination_id")
		if err != nil {
			fmt.Fprintf(w, "%s", httpError(err))
			return
		}

		fmt.Printf("[%s]: PlayerId: %d is sending SCV [%d-->%d]\n", time.Now(), playerID, locID, destID)
		err = sendSCV(playerID, locID, destID)
		if err != nil {
			fmt.Fprintf(w, "%s", httpError(err))
			return
		}
		fmt.Fprintf(w, "%s", `{"status":"ok"}`)
		return
	}

	fmt.Fprintf(w, "%s", `{"error":{"message":"Not supported action"}}`)
}

func sendSCV(playerID int, locID int, destID int) error {
	testGame.objectsMu.Lock()
	defer testGame.objectsMu.Unlock()
	for i, gob := range testGame.Objects {
		if gob.Type == GAME_UNIT_SCV && gob.Owner == playerID && gob.Location == locID && gob.Status == STATUS_IDLE {
			testGame.Objects[i].Location = destID
			return nil
		}
	}
	return fmt.Errorf("Couldn't find any IDLE SCVs at location %d for player %d", locID, playerID)
}

func statusSCV(playerID int, locID int, status_from int, status_to int) error {
	testGame.objectsMu.Lock()
	defer testGame.objectsMu.Unlock()
	for i, gob := range testGame.Objects {
		if gob.Type == GAME_UNIT_SCV && gob.Owner == playerID && gob.Location == locID && gob.Status == status_from {
			testGame.Objects[i].Status = status_to
			return nil
		}
	}
	if status_from == STATUS_MINING {
		return fmt.Errorf("Couldn't find any MINING SCVs at location %d for player %d", locID, playerID)
	}
	return fmt.Errorf("Couldn't find any IDLE SCVs at location %d for player %d", locID, playerID)
}

func buildSCV(playerID int, locID int) error {
	if playerID != locID {
		return fmt.Errorf("No command center at location %d, my base location is %d", locID, playerID)
	}
	pl := &testGame.Players[playerID]
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if pl.Minerals < COST_SCV_MINERALS {
		return fmt.Errorf("Not enogh minerals, need %d, have %d", COST_SCV_MINERALS, pl.Minerals)
	}
	pl.Minerals -= COST_SCV_MINERALS

	testGame.objectsMu.Lock()
	defer testGame.objectsMu.Unlock()
	testGame.Objects = append(testGame.Objects, SCV(playerID, playerID))
	return nil
}

func httpError(err error) string {
	return fmt.Sprintf(`{"error":{"message":"%s"}}`, strings.Replace(err.Error(), `"`, `\"`, -1))
}
