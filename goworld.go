// GoWorld is a simulator of life. It places persons on a map and watches them how they
// move randomly around and interact with each other
//
// @author David Rubin, 2020
package GoWorld

import (
	"github.com/google/uuid"
	"image"
	"image/color"
)

// Location represents coordinates of an object
type Location struct {
	X, Y int // The lower left corner is deemed as 0,0, X travels horizontally and Y vertically
}

// Being is a living creature that is 'living' on the terrain
type Being struct {
	ID             uuid.UUID // The identifier
	Hunger         float64   // The desire for food
	Thirst         float64   // The desire for liquid
	WantsChild     float64   // The desire to produce offspring
	LifeExpectancy float64   // How many epochs the being will survive
	VisionRange    float64   // How far the creature can spot objects
	Speed          float64   // How fast the creature can move (faster -> get hungry and thirsty quicker)
	Durability     float64   // More durable creatures need less food and liquids
	Stress         float64   // How stressed the creature is
	// Stress increases when:
	// 	- the being becomes hungrier / thirstier
	//  - does not find a partner to reproduce
	// 	- does not feel safe, e.g. is travelling outside its habitat
	Habitat uuid.UUID // The natural habitat where this creature can be found
	Gender  string    // The gender of the creature
	Size    float64   // Physical size of the creature (bigger need more food and liquid, but are
	// not affected by stress as much)
	Fertility float64 // The number of offspring produced after successful mating with another being
	// The offspring inherit their features from the parents with a random value using the parents values as borders
	MutationRate float64  // How much the attributes can deviate
	Position     Location // Where the creature is currently located in the world
	// The creature can not move on water (Jesus not implemented yet) or on mountain peaks.
}

// Food is for now just plants
type Food struct {
	GrowthSpeed     float64 // How fast the food will grow (how many epochs to move to the next growth stage)
	NutritiousValue float64 // How much it decreases the hunger (also possible for minimal thirst decrease)
	Taste           float64 // Tastier food is preferred among creatures (when not too hungry)
	GrowthStage     float64 // The current growth phase of the food
	Area            float64 // How much area it needs to grow
	Fertility       float64 // How many offspring can be produced
	LifeExpectancy  float64 // How many epochs it can survive
	Habitat         float64 // The natural habitat of the plant
	// Aditional rules for plants:
	//  - a plant has 4 growth stages (each stage has the portion of the defined features, e.g. 25%, 50%, 75%, 100%)
	//  - beings prefer older plants (if they are not too hunrgy)
	//  - when the plant evolves to the next stage it can reproduce (e.g. when evolving to the second stage it can throw
	//    25% - 50% of its seeds)
}

// World is an interface to construct and manage the world with beings (terrain and such)
type World interface {
	New() error // create a new world (terrain + creatures + items)

	// Getters
	GetTerrainImage() *image.RGBA                   // Returns the colored terrain as an image
	GetBeings() []*Being                            // Returns all beings currently living in the world
	GetSurfaceColorAtSpot(spot Location) color.RGBA // Returns the color of the surface at a location

	CreateBeings(quantity int) []*Being // Create random beings and place them (previous beings should remain)
	CreateRandomBeing() *Being          // Make a random being (predefined attribute ranges)
	ThrowBeing(b *Being)                // Place the (NEW) being onto a random map (adjusts its habitat to that spot)
	MoveBeing(b *Being) error           // Make the provided being move randomly across the terrain
}
