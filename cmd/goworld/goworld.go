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
	world.CreateCarnivores(15)
	world.CreateFishies(10)
	world.CreateFlyers(15)
	// Add food
	world.ProvideFood(30, 20)

	// Run the animation
	display.Run(world)
}
