package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"

	"golang.org/x/exp/slices"
)

type Channels struct {
	Channels []Channel `json:"channels"`
}

type Channel struct {
	ID string `json:"id"`
}

const DATAFILE = "data.json"

var channels Channels

func init() {
	file, err := os.OpenFile(DATAFILE, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	file.Close()
	text, err := os.ReadFile(DATAFILE)
	if err != nil {
		fmt.Println("error reading data file,", err)
		os.Exit(1)
	}

	json.Unmarshal(text, &channels)

}

func writeJSON() {
	text, err := json.Marshal(channels)
	if err != nil {
		fmt.Println("error marshaling data,", err)
	}
	os.WriteFile(DATAFILE, text, fs.FileMode(os.O_RDWR))
}

func addChannel(channelID string) bool {
	if isChannelActivated(channelID) {
		return false
	}
	channels.Channels = append(channels.Channels, Channel{ID: channelID})
	writeJSON()
	fmt.Printf("added channel %v\n", channelID)
	return true
}

func removeChannel(channelID string) bool {
	if !isChannelActivated(channelID) {
		return false
	}
	index := slices.Index(channels.Channels, Channel{ID: channelID})
	channels.Channels = slices.Delete(channels.Channels, index, index+1)
	writeJSON()
	fmt.Printf("removed channel %v\n", channelID)
	return true
}

func isChannelActivated(channelID string) bool {
	return slices.Contains(channels.Channels, Channel{ID: channelID})
}
