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
	world.CreateCarnivores(100)
	//world.CreateFishies(10)
	world.CreateFlyers(100)
	// Add food
	world.ProvideFood(50, 10)

	// Run the animation
	display.Run(world)
}
