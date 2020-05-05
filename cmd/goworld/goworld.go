package main

import (
	"bufio"
	"encoding/json"
	"github.com/rubinda/GoWorld/display"
	"github.com/rubinda/GoWorld/terrain"
	"os"
)

const (
	width  = 1000
	height = 1000
)

func main() {
	// Initialize a world
	world := &terrain.RandomWorld{
		Width: width, Height: height,
	}
	// Create the terrain
	_ = world.New()
	// Add beings
	world.CreateBeings(1)
	// Add food
	world.ProvideFood(20)
	bgs := world.GetBeings()

	fi, _ := os.Create("beingList.json")
	defer fi.Close()
	fz := bufio.NewWriter(fi)
	e := json.NewEncoder(fz)

	if err := e.Encode(bgs); err != nil {
		panic(err)
	}

	// Run the animation
	display.Run(world)
}
