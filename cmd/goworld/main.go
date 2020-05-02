package main

import (
	"github.com/rubinda/GoWorld/display"
	"github.com/rubinda/GoWorld/terrain"
)

func main() {
	world := &terrain.RandomWorld{
		Width: 1000, Height: 1000,
	}
	world.New()
	world.CreateBeings(10)
	display.Run(1000, 1000, world)
}
