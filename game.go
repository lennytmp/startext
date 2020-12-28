package main

import (
	"sync"
	"time"
    "log"
    "math/rand"
	"encoding/json"
	"fmt"
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

func gameSim(g *Game) {
	killedIDs := make(map[int]bool)
	for i, gob := range g.Objects {
		if gob.Status == STATUS_MINING {
			g.Players[gob.Owner].Minerals += gob.yps
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
	g.lastSim = time.Now()
	for k := range g.Players {
		_, ok := buildingsPerPlayer[k]
		if !ok {
			g.Players[k].Outcome = ELIMINATED
		} else if len(buildingsPerPlayer) == 1 {
			g.status = GAME_STATUS_FINISHED
			g.Players[k].Outcome = VICTORY
		}
	}
}

func initGame(g *Game) {
	g.status = GAME_STATUS_RUNNING
	l := 0
	for n, pl := range g.Players {
		g.Locations = append(g.Locations, Location{})
		pl.Minerals = 50
		for j := 0; j < 5; j++ {
			g.Objects = append(g.Objects, SCV(n, l))
		}
		g.Objects = append(g.Objects, CommandCenter(n, l))
		l++
	}
	log.Printf("Game %s started", g.name)
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
	Ready    bool
	bot      bool
}

type Game struct {
	Players   map[string]*Player
	Locations []Location
	Objects   []GameObject
	lastSim   time.Time
	status    string
	name      string
	mu        sync.Mutex
}

func newGame(gameName string) *Game {
	g := &Game{}
	g.Players = make(map[string]*Player)
	g.status = GAME_STATUS_PENDING
	g.lastSim = time.Now()
	g.name = gameName
	return g
}

func (g Game) exportAll() string {
	b, err := json.Marshal(g)
	if err != nil {
		log.Printf("ERROR json.Marshal for the game %v %v", g, err)
		return ""
	}
	return string(b)
}

func (g Game) Export(player string) string {
	if g.status != GAME_STATUS_RUNNING {
		return g.exportAll()
	}
	eg := newGame(g.name)
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
	return eg.exportAll()
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

func (g Game) String() string {
	b, err := json.Marshal(g)
	if err != nil {
		log.Printf("ERROR json.Marshal %v", err)
		return ""
	}
	return string(b)
}

func sendSCV(g *Game, player string, locID int, destID int) error {
	for i, gob := range g.Objects {
		if gob.Type == GAME_UNIT_SCV && gob.Owner == player && gob.Location == locID && gob.Status == STATUS_IDLE {
			g.Objects[i].Location = destID
			return nil
		}
	}
	return fmt.Errorf("couldn't find any IDLE SCVs at location %d for player %s", locID, player)
}

func statusSCV(g *Game, player string, locID int, status_from int, status_to int) error {
	for i, gob := range g.Objects {
		if gob.Type == GAME_UNIT_SCV && gob.Owner == player && gob.Location == locID && gob.Status == status_from {
			g.Objects[i].Status = status_to
			return nil
		}
	}
	if status_from == STATUS_MINING {
		return fmt.Errorf("couldn't find any MINING SCVs at location %d for player %s", locID, player)
	}
	return fmt.Errorf("couldn't find any IDLE SCVs at location %d for player %s", locID, player)
}

func buildSCV(g *Game, player string, locID int) error {
	ccFound := false
	var ccID int
	for i, gob := range g.Objects {
		if gob.Type == GAME_BUILDING_COMMAND_CENTER && gob.Location == locID && gob.Owner == player {
			ccFound = true
			ccID = i
		}
	}
	if !ccFound {
		return fmt.Errorf("no command center at location %d", locID)
	}
	if g.Objects[ccID].Building.Task != (Task{}) {
		return fmt.Errorf("the command center is busy, sorry")
	}
	pl := g.Players[player]
	if pl.Minerals < COST_SCV_MINERALS {
		return fmt.Errorf("not enogh minerals, need %d, have %d", COST_SCV_MINERALS, pl.Minerals)
	}
	pl.Minerals -= COST_SCV_MINERALS
	g.Objects[ccID].Building.Task = Task{Type: TASK_TYPE_BUILD_SCV}
	return nil
}
