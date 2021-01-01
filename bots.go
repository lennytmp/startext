package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type triggerRequest struct {
	t       time.Time
	game    string
	botName string
}

var (
	botTriggerQueue           chan triggerRequest
	makeBotRequestOverridable = makeBotRequest
)

func processBotQueue() {
	tr := <-botTriggerQueue
	if time.Now().After(tr.t) {
		triggerBot(tr.game, tr.botName)
	} else {
		botTriggerQueue <- tr
	}
}

func makeBotRequest(url string) ([]byte, error) {
	var res []byte
	resp, err := http.Get("http://localhost:8182" + url)
	if err != nil {
		return res, fmt.Errorf("sending request failed %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return res, fmt.Errorf("bad status code: %v", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return res, fmt.Errorf("couldn't read all response body %v", err)
	}
	return body, nil
}

func triggerBot(gameName string, botName string) {
	{ // Get state and process it
		rURL := fmt.Sprintf("/?player=%s", botName)
		resp, err := makeBotRequestOverridable(rURL)
		if err != nil {
			log.Printf("ERROR: Making request %s for bot %s game %s failed with %v", rURL, botName, gameName, err)
		}
		g := &Game{}
		err = json.Unmarshal(resp, g)
		if err != nil {
			log.Printf("ERROR: Bot %s got resp %s from request %s for game %s, but couldn't transform it to Game: %v", botName, resp, rURL, gameName, err)
		}
		minerals := g.Players[botName].Minerals
		homeId := 0
		var commandCenter *GameObject
		perLocOwner := make(map[int]map[string]map[string]int)
		for j, gob := range g.Objects {
			if gob.Owner == botName && gob.Building.Type == BUILDING_COMMAND_CENTER {
				homeId = gob.Location
				commandCenter = &g.Objects[j]
				continue
			}
			if gob.Type == OBJECT_BUILDING {
				continue
			}
			if _, ok := perLocOwner[gob.Location]; !ok {
				perLocOwner[gob.Location] = make(map[string]map[string]int)
				perLocOwner[gob.Location][gob.Owner] = make(map[string]int)
			}
			if _, ok := perLocOwner[gob.Location][gob.Owner]; !ok {
				perLocOwner[gob.Location][gob.Owner] = make(map[string]int)
			}
			if _, ok := perLocOwner[gob.Location][gob.Owner][gob.Unit.Status]; !ok {
				perLocOwner[gob.Location][gob.Owner][gob.Unit.Status] = 0
			}
			perLocOwner[gob.Location][gob.Owner][gob.Unit.Status]++
		}
		if len(perLocOwner[homeId]) > 1 {
			// We are under attack
			for i := 0; i < perLocOwner[homeId][botName][UNIT_STATUS_MINING]; i++ {
				rURL := fmt.Sprintf("/?player=%s&location_id=%d&idle_scv", botName, homeId)
				_, err := makeBotRequestOverridable(rURL)
				if err != nil {
					log.Printf("ERROR: Making request %s for bot %s game %s failed with %v", rURL, botName, gameName, err)
				}
			}
		} else {
			for i := 0; i < perLocOwner[homeId][botName][UNIT_STATUS_IDLE]; i++ {
				rURL := fmt.Sprintf("/?player=%s&location_id=%d&scv_to_work", botName, homeId)
				_, err := makeBotRequestOverridable(rURL)
				if err != nil {
					log.Printf("ERROR: Making request %s for bot %s game %s failed with %v", rURL, botName, gameName, err)
				}
			}
		}
		if minerals >= 50 && commandCenter.Task == (Task{}) {
			rURL := fmt.Sprintf("/?player=%s&location_id=%d&build_scv", botName, homeId)
			_, err := makeBotRequestOverridable(rURL)
			if err != nil {
				log.Printf("ERROR: Making request %s for bot %s game %s failed with %v", rURL, botName, gameName, err)
			}
		}
	}
	botTriggerQueue <- triggerRequest{time.Now().Add(30 * time.Second), gameName, botName}
}
