package main

import (
	"github.com/rubinda/GoWorld/display"
	"github.com/rubinda/GoWorld/terrain"
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
	world.CreateBeings(28)
	// Add food
	world.ProvideFood(100)

	// Run the animation
	display.Run(world)
}
