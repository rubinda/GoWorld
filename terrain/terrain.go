package terrain

import (
	. "bufio"
	. "encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/rubinda/GoWorld"
	"github.com/rubinda/GoWorld/noise"
	"github.com/rubinda/GoWorld/pathing"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"os"
	"time"
)

var (
	// Surfaces are the currently predefined surface types (the 'elevation zones' of the terrain)
	Surfaces = []Surface{
		{uuid.New(), "Water", color.RGBA{R: 116, G: 167, B: 235, A: 255}, false},
		{uuid.New(), "Grassland", color.RGBA{R: 96, G: 236, B: 133, A: 255}, true},
		{uuid.New(), "Forest", color.RGBA{R: 44, G: 139, B: 54, A: 255}, true},
		{uuid.New(), "Gravel", color.RGBA{R: 198, G: 198, B: 198, A: 255}, true},
		{uuid.New(), "Mountain", color.RGBA{R: 204, G: 153, B: 102, A: 255}, true},
		{uuid.New(), "Moutain Peak", color.RGBA{R: 240, G: 240, B: 240, A: 255}, false},
	}
	// Used when converting HEX color to RGB
	errInvalidFormat = errors.New("invalid HEX string format")

	// The ranges of being attributes used when randomly generating a new being
	// Todo move these ranges to a config file (or better yet allow the user to specify them)
	hungerRange         = &attributeRange{0, 255}
	thirstRange         = &attributeRange{0, 255}
	wantsChildRange     = &attributeRange{0, 255}
	lifeExpectancyRange = &attributeRange{1, 64}
	visionRange         = &attributeRange{1, 64}
	speedRange          = &attributeRange{1, 16}
	durabilityRange     = &attributeRange{0, 255}
	stressRange         = &attributeRange{0, 255}
	sizeRange           = &attributeRange{0, 64}
	fertilityRange      = &attributeRange{0, 4}
	mutationRange       = &attributeRange{0, 31}

	// Attribute ranges for food
	growthRange        = &attributeRange{0, 15}
	nutritionRange     = &attributeRange{0, 128}
	tasteRange         = &attributeRange{0, 255}
	stageRange         = &attributeRange{0, 3}
	stageProgressRange = &attributeRange{0, 255}
	areaRange          = &attributeRange{4, 32}
	seedRange          = &attributeRange{3, 8}
	witherRange        = &attributeRange{1, 256}
	disperseRange      = &attributeRange{1, 8}

	// Being thresholds for action
	hungerThreshold = 150.
	stressThreshold = 175.
	// Being increments for basic necessities
	hungerIncrease     = 0.2
	thirstIncrease     = 0.3
	wantsChildIncrease = 0.05
	// Movespeed of water plants (is fixed)
	seaweedMoveSpeed = 3

	// Adjacent directions without the center point
	directions8 = [8]GoWorld.Location{
		{-1, -1},
		{0, -1},
		{1, -1},
		{1, 0},
		{1, 1},
		{0, 1},
		{-1, 1},
		{-1, 0},
	}

	// Adjacent fields and center point
	directions9 = [9]GoWorld.Location{
		{0, 0},
		{-1, -1},
		{0, -1},
		{1, -1},
		{1, 0},
		{1, 1},
		{0, 1},
		{-1, 1},
		{-1, 0},
	}
)

// RandomWorld represents the world implementation using Perlin Noise as terrain
type RandomWorld struct {
	// TODO introduce MaxBeings based on world size / terrain
	Width, Height int
	TerrainImage  *image.Gray // TerrainImage holds the terrain surface image (like a DEM model)
	TerrainZones  *image.RGBA // TerrainZones is a colored version of TerrainImage (based on defined zones and ratios)
	TerrainSpots  [][]*Spot   // TerrainSpots holds data about each spot on the map (what surface, what object or being
	// occupies it)
	BeingList  map[string]*GoWorld.Being // The list of world inhabitants
	FoodList   map[string]*GoWorld.Food  // List of all edible food
	pathFinder GoWorld.Pathfinder
}

// Spot is a place on the map with a defined surface type.
// Optionally an object (e.g. food) and a being can be located in it (a being above the object, for example eating food)
type Spot struct {
	Surface        *Surface  // The surface attributes
	Object         uuid.UUID // The UUID for the object (nil for nothing) visible on surface
	Being          uuid.UUID // The being on the spot (nil for noone)
	OccupyingPlant uuid.UUID // The plant using this spot for growth (see Food.Area) not necessarily visible on surface
	// if this is nil, a plant can be placed here (given enough room around for its area)
}

// Surface represents the data about a certain zone
type Surface struct {
	ID         uuid.UUID // The UUID (e.g. '7d444840-9dc0-11d1-b245-5ffdce74fad2'
	CommonName string    // A common name for it (e.g. 'Forest')
	// TODO use textures instead of colors
	Color     color.RGBA // A color value for the appearance
	Habitable bool       // Whether a Being can move across this surface (e.g. Can't walk on moutain peaks or on
	// water) or if a plant can grow here
}

// attributeRange is used to define the minimum and maximum value of an attribute
type attributeRange struct {
	Min float64
	Max float64
}

// randomFloat returns a random floating point number for the given attribute range
func (r *attributeRange) randomFloat() float64 {
	return rand.Float64()*r.Max + r.Min
}

// randomInt returns a random integer value from the range
func (r *attributeRange) randomInt() int {
	return int(rand.Float64()*r.Max + r.Min)
}

// randomGender picks a gender with a 50/50 chance
func randomGender() string {
	rand.Seed(time.Now().UnixNano())
	coinFlip := rand.Intn(2)
	if coinFlip > 0 {
		return "female"
	}
	return "male"

}

// PlantsToJSON stores the current edible plants in the world into a json file
func (w *RandomWorld) PlantsToJSON(fileName string) {
	fi, _ := os.Create(fileName)
	defer fi.Close()
	fz := NewWriter(fi)
	e := NewEncoder(fz)

	if err := e.Encode(w.FoodList); err != nil {
		panic(err)
	}
}

// BeingsToJSON stores the current living beings to a file
func (w *RandomWorld) BeingsToJSON(fileName string) {
	fi, _ := os.Create(fileName)
	defer fi.Close()
	fz := NewWriter(fi)
	e := NewEncoder(fz)

	if err := e.Encode(w.BeingList); err != nil {
		panic(err)
	}
}

// IsOutOfBounds check if a location is inside the terrain zone. Returns true if location outside the bounds.
func (w *RandomWorld) IsOutOfBounds(location GoWorld.Location) bool {
	if location.X < 0 || location.X >= w.Width || location.Y < 0 || location.Y >= w.Height {
		return true
	}
	return false
}

// ParseHexColorFast converts HEX color string to RGBA.
// All of this code was found on Stack Overflow. Thanks to @icza (https://stackoverflow.com/a/54200713)
func ParseHexColorFast(s string) (c color.RGBA, err error) {
	c.A = 0xff
	if s[0] != '#' {
		return c, errInvalidFormat
	}
	hexToByte := func(b byte) byte {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		}
		err = errInvalidFormat
		return 0
	}
	switch len(s) {
	case 7:
		c.R = hexToByte(s[1])<<4 + hexToByte(s[2])
		c.G = hexToByte(s[3])<<4 + hexToByte(s[4])
		c.B = hexToByte(s[5])<<4 + hexToByte(s[6])
	case 4:
		c.R = hexToByte(s[1]) * 17
		c.G = hexToByte(s[2]) * 17
		c.B = hexToByte(s[3]) * 17
	default:
		err = errInvalidFormat
	}
	return
}

// CalculateZoneLimits returns the upper bound values for zones if ratios are given of how much area each zone covers
// The number of zones can vary but sum(ratios) must equal to 1.0
func (w *RandomWorld) CalculateZoneLimits(hist []int, ratios ...float64) []uint8 {
	// Check if ratios add up to ~1 (otherwise parts of the terrain / zones will be left untouched)
	var binSum float64
	for _, r := range ratios {
		binSum += r
	}
	// Check if larger than 1.00..001 (added a delta of 1e-14 because of precision error ...)
	if binSum > 1.0+1e-14 || binSum < 1.0-1e-14 {
		panic(fmt.Errorf("error while calculating zone limits: the given ratios (%v) do not add up to 1 (%v)",
			ratios, binSum))
	}
	// Check if enough surfaces (zones) are defined
	if len(Surfaces) < len(ratios) {
		panic(fmt.Errorf("error while calculating zone limits: given %d ratios, but only %d Surfaces defined",
			len(ratios), len(Surfaces)))
	}
	// The limits are based on 8bit grayscale
	limits := make([]uint8, len(ratios))

	// Count how many pixels lie in each bin and add up bins until the bin pixel to all pixel ratio is as close
	// to the wanted one as possible
	currentBin := 0
	var previousSum float64
	allPixels := float64(w.Width * w.Height)
	for i, r := range ratios {
		binSum = float64(hist[currentBin])
		// Increase the bin count until we reach the tipping point
		for binSum/allPixels < r {
			currentBin++
			if currentBin > 255 {
				// The last zone has < wanted ratio pixels, stop so we don't get out of range
				currentBin--
				break
			}
			binSum += float64(hist[currentBin])
		}

		// Check if the previous bins (ratio <= wanted ratio) were closer to the wanted ratio than the current sum
		// (ratio > wanted ratio). Choose the one that is closer
		previousSum = binSum - float64(hist[currentBin])
		if math.Abs(previousSum/allPixels-r) <= math.Abs(binSum/allPixels-r) {
			// The previous bin sum was closer (or at the same distance) than the current sum, prefer less bins
			currentBin--
		}
		// Store the current bin index as the limiting factor
		limits[i] = uint8(currentBin)
		// Move on to calculate the next zone limit
		currentBin++
	}
	// The last limit should always be the highest value. Because we do not produce perfectly matched ratios some zones
	// can take up more space (or less) and it causes the last zone to undershoot its upper border. In a perfect
	// scenario it should already be 255
	limits[len(limits)-1] = 255
	return limits
}

// CreateCarnivores generates instances of beings and fills them with random attributes
// Provide the number of beings to create
// Note that the beings are added to the world and previously created beings are kept
func (w *RandomWorld) CreateCarnivores(quantity int) {
	// Initialize each being to a random one
	for i := 0; i < quantity; i++ {
		// Create random being and place it into the map
		b := w.CreateRandomCarnivore()
		w.BeingList[b.ID.String()] = b
	}
}

// CreateFishies generates random instances of beings that live in water
func (w *RandomWorld) CreateFishies(quantity int) {
	// Initialize each being to a random one
	for i := 0; i < quantity; i++ {
		// Create random being and place it into the map
		b := w.CreateRandomFish()
		w.BeingList[b.ID.String()] = b
	}
}

// CreateFlyers generates instances of random flying beings
func (w *RandomWorld) CreateFlyers(quantity int) {
	// Initialize each being to a random one
	for i := 0; i < quantity; i++ {
		// Create random being and place it into the map
		b := w.CreateRandomFlyer()
		w.BeingList[b.ID.String()] = b
	}
}

// CreateRandomCarnivore returns a new being with random parameters (places it onto the map)
func (w *RandomWorld) CreateRandomCarnivore() *GoWorld.Being {
	// Create an empty being
	being := &GoWorld.Being{ID: uuid.New()}
	being.Type = "Carnivore"

	// Give the being the basic necessities
	being.Hunger = hungerRange.randomFloat()
	being.Thirst = thirstRange.randomFloat()
	being.WantsChild = wantsChildRange.randomFloat()

	// Shape the being
	being.LifeExpectancy = lifeExpectancyRange.randomFloat()
	being.VisionRange = visionRange.randomFloat()
	being.Speed = speedRange.randomFloat()
	being.Durability = durabilityRange.randomFloat()
	being.Stress = stressRange.randomFloat()
	being.Size = sizeRange.randomFloat()
	being.Gender = randomGender()
	being.Fertility = fertilityRange.randomFloat()
	being.MutationRate = mutationRange.randomFloat()

	// Pick a random (valid) position and check which habitat it is
	w.ThrowBeing(being)

	return being
}

// CreateRandomFlyer generate an instance of a being that can fly
func (w *RandomWorld) CreateRandomFlyer() *GoWorld.Being {
	// Create an empty being
	being := &GoWorld.Being{ID: uuid.New()}
	being.Type = "Flying"

	// Give the being the basic necessities
	being.Hunger = hungerRange.randomFloat()
	being.Thirst = thirstRange.randomFloat()
	being.WantsChild = wantsChildRange.randomFloat()

	// Shape the being
	being.LifeExpectancy = lifeExpectancyRange.randomFloat()
	being.VisionRange = visionRange.randomFloat()
	being.Speed = speedRange.randomFloat()
	being.Durability = durabilityRange.randomFloat()
	being.Stress = stressRange.randomFloat()
	being.Size = sizeRange.randomFloat()
	being.Gender = randomGender()
	being.Fertility = fertilityRange.randomFloat()
	being.MutationRate = mutationRange.randomFloat()

	// Flying beings 'feel' home in the forest, but can spawn anywhere
	// Create some random coordinates within the world limits
	rX := rand.Intn(w.Width)
	rY := rand.Intn(w.Height)
	overflow := 0
	// If no being present at location set it as the spawn point
	for w.TerrainSpots[rX][rY].Being != uuid.Nil {
		rX = rand.Intn(w.Width)
		rY = rand.Intn(w.Height)
		// Recover somehow if we look for a location for too long
		overflow++
		if overflow > 100000 {
			// Todo handle the infinite loop a little nicer than panicking
			panic("error placing flying being: tried 100k random spots and all occupied")
		}
	}
	being.Position.X = rX
	being.Position.Y = rY
	being.Habitat = Surfaces[2].ID

	return being
}

// CreateRandomFish generates an instance of a being that lives in water
func (w *RandomWorld) CreateRandomFish() *GoWorld.Being {
	// Create an empty being
	being := &GoWorld.Being{ID: uuid.New()}
	being.Type = "Water"

	// Give the being the basic necessities
	being.Hunger = hungerRange.randomFloat()
	being.Thirst = thirstRange.randomFloat()
	being.WantsChild = wantsChildRange.randomFloat()

	// Shape the being
	being.LifeExpectancy = lifeExpectancyRange.randomFloat()
	being.VisionRange = visionRange.randomFloat()
	being.Speed = speedRange.randomFloat()
	being.Durability = durabilityRange.randomFloat()
	being.Stress = stressRange.randomFloat()
	being.Size = sizeRange.randomFloat()
	being.Gender = randomGender()
	being.Fertility = fertilityRange.randomFloat()
	being.MutationRate = mutationRange.randomFloat()

	// Water beings should spawn in water
	rX := rand.Intn(w.Width)
	rY := rand.Intn(w.Height)
	overflow := 0
	// If no being present at location set it as the spawn point
	for w.TerrainSpots[rX][rY].Surface.CommonName != "Water" && w.TerrainSpots[rX][rY].Being == uuid.Nil {
		rX = rand.Intn(w.Width)
		rY = rand.Intn(w.Height)
		// Recover somehow if we look for a location for too long
		overflow++
		if overflow > 100000 {
			// Todo handle the infinite loop a little nicer than panicking
			panic("error placing water being: tried 100k random spots and all occupied / not water")
		}
	}
	being.Position.X = rX
	being.Position.Y = rY
	// Should always be water ID
	being.Habitat = w.TerrainSpots[rX][rY].Surface.ID

	return being
}

// ThrowBeing randomly places the a being onto the map (onto walkable surfaces)
// Use with caution as it adjusts the beings habitat to that spot
func (w *RandomWorld) ThrowBeing(b *GoWorld.Being) {
	// Check if the terrain to place the being exists
	if w.TerrainSpots == nil {
		panic(fmt.Errorf("error while creating being: no terrain to place being on"))
	}

	// Create some random coordinates within the world limits
	randomSpot := GoWorld.Location{}
	randomSpot.X = rand.Intn(w.Width)
	randomSpot.Y = rand.Intn(w.Height)

	// Check if the chosen spot was valid (no being already present and surface is walkable)
	// If not repeat the random process until we find a suitable spot
	for !w.canPlaceBeing(randomSpot, b.Type) {
		randomSpot.X = rand.Intn(w.Width)
		randomSpot.Y = rand.Intn(w.Height)
	}
	// Set the location of the being
	b.Position.X = randomSpot.X
	b.Position.Y = randomSpot.Y
	w.TerrainSpots[randomSpot.X][randomSpot.Y].Being = b.ID

	// Specify into which habitat (surface type) it falls
	b.Habitat = w.TerrainSpots[randomSpot.X][randomSpot.Y].Surface.ID
}

// ThrowPlant randomly places a plant (food) onto the map
func (w *RandomWorld) ThrowPlant(p *GoWorld.Food) {
	// Check if the terrain to place the plant exists
	if w.TerrainSpots == nil {
		panic(fmt.Errorf("error while throwing plant: no terrain"))
	}

	// Create some random coordinates within the world limits
	rX := rand.Intn(w.Width)
	rY := rand.Intn(w.Height)

	for !w.canPlacePlant(rX, rY, p.Area) {
		rX = rand.Intn(w.Width)
		rY = rand.Intn(w.Height)
	}
	// Place the plant on the surface and occupy spots in area
	w.updatePlantSpot(rX, rY, p.Area, p.ID)
	p.Position.X = rX
	p.Position.Y = rY
}

// LaunchPlant randomly places a plant onto water (will become a water plant)
func (w *RandomWorld) LaunchPlant(p *GoWorld.Food) {
	// Check if the terrain to place the being exists
	if w.TerrainSpots == nil {
		panic(fmt.Errorf("error while launching water plant: no terrain"))
	}
	// Create some random coordinates within the world limits
	rX := rand.Intn(w.Width)
	rY := rand.Intn(w.Height)

	for !w.canPlaceWaterPlant(rX, rY, p.Area, p.ID) {
		rX = rand.Intn(w.Width)
		rY = rand.Intn(w.Height)
	}
	// Place the plant on the surface and occupy spots in area
	w.updatePlantSpot(rX, rY, p.Area, p.ID)
	p.Position.X = rX
	p.Position.Y = rY
}

// UpdateBeing executes the next action for the being
// Returns action done as string and UUIDs of objects affected by action
func (w *RandomWorld) UpdateBeing(b *GoWorld.Being) (string, []uuid.UUID) {
	// Check if it is time for the being to die
	if b.LifeExpectancy <= 0 || b.Thirst >= 255 || b.Hunger >= 255 {
		// Being has reached EOL
		fmt.Printf("Being (%v) %v ", b.Type, b.ID)
		if b.LifeExpectancy <= 0 {
			fmt.Println("... died of old age")
		} else if b.Thirst >= 255 {
			fmt.Println("... died of thirst")
		} else if b.Hunger >= 255 {
			fmt.Println("... died of hunger")
		}
		// remove being from BeingList & TerrainSpots
		delete(w.BeingList, b.ID.String())
		w.TerrainSpots[b.Position.X][b.Position.Y].Being = uuid.Nil
		return "died", []uuid.UUID{b.ID}
	}
	// Increase the age (=> lower life expectancy for 1 epoch)
	b.LifeExpectancy -= 1. / 60 // Age roughly every second (60 FPS)
	actionDone := "wandered"
	var objectsAffected []uuid.UUID
	actionToDo, actionSpot := w.SenseActionFor(b)
	allowInhabitable := false
	if b.Type == "Water" || b.Type == "Flying" {
		allowInhabitable = true
	}

	pathToAction := w.pathFinder.GetPath(b.Position, actionSpot, allowInhabitable)
	// Whether carnivore beings successfully ate
	successfulHunt := false
	if len(pathToAction) == 0 {
		// Todo investigate which paths are not found
	}
	switch actionToDo {
	case "drink":
		// Check if being has to move to take the action
		if int(b.Speed) >= len(pathToAction) || b.Type == "Water" {
			// We are fast enough to get to action spot in one move
			if len(pathToAction) >= 1 {
				w.MoveBeingToLocation(b, pathToAction[len(pathToAction)-1])
			}
			w.QuenchThirst(b)
		} else {
			// We see further than we can move in one epoch
			w.MoveBeingToLocation(b, pathToAction[int(b.Speed)])
		}
		actionDone = "drank"
	case "eat":
		if int(b.Speed) >= len(pathToAction) {
			// We are fast enough to get to action spot in one move
			if len(pathToAction) >= 1 {
				w.MoveBeingToLocation(b, pathToAction[len(pathToAction)-1])
			}
			if (b.Type == "Flying" || b.Type == "Carnivore") &&
				w.TerrainSpots[actionSpot.X][actionSpot.Y].Being != uuid.Nil {
				// We are eating a being, rename action done accordingly
				actionDone = "ate being"
				objectsAffected = append(objectsAffected, w.TerrainSpots[actionSpot.X][actionSpot.Y].Being)
				//fmt.Printf("Being (%v) %v ate being\n", b.Type, b.ID)
			} else {
				// We are eating a plant
				objectsAffected = append(objectsAffected, w.TerrainSpots[actionSpot.X][actionSpot.Y].OccupyingPlant)
				//fmt.Printf("Being (%v) %v ate plant\n", b.Type, b.ID)
				actionDone = "ate plant"
			}
			w.QuenchHunger(b, actionSpot)
			// Carnivore Being ate, so lower speed before stress update
			if b.Type == "Carnivore" {
				b.Speed /= 2
				successfulHunt = true
			}
		} else {
			// We see further than we can move in one epoch
			w.MoveBeingToLocation(b, pathToAction[int(b.Speed)])
			actionDone = "ate fail"
		}

	case "mate":
		if int(b.Speed) >= len(pathToAction) {
			// We are fast enough to get to action spot in one move
			if len(pathToAction) >= 1 {
				w.MoveBeingToLocation(b, pathToAction[len(pathToAction)-1])
			}
			objectsAffected = append(objectsAffected, w.MateBeing(b)...)
			actionDone = "mated"
		} else {
			// We see further than we can move in one epoch
			w.MoveBeingToLocation(b, pathToAction[int(b.Speed)])
		}
	case "wander":
		w.MoveBeingToLocation(b, actionSpot)
		actionDone = "wandered"
	case "hold":
		// Do nothing, we cannot move to any surrounding spot inside vision range
		actionDone = "froze"
	}

	// Update stress:
	//  increase for higher thirst, hunger and the wish to reproduce, out of natural habitat
	//  lower for higher size, durability
	w.AdjustStressFor(b)
	w.AdjustNeeds(b)

	// If being didn't eat this round (speed lower than vision range), reset the hunting speed after needs and stress
	// update
	if b.Type == "Carnivore" && actionDone == "ate fail" && !successfulHunt {
		b.Speed /= 2
	}
	return actionDone, objectsAffected
}

// Wander moves a being similar to Brownian Motion
// Implementation reference: http://people.bu.edu/andasari/courses/stochasticmodeling/lecture5/stochasticlecture5.html
// I have adjusted the following parameters:
//  - the time step (delta t) is the speed of each creature
//  - the previous position is the current position of the being
//  - the next position is recalculated until a valid one is found
func (w *RandomWorld) Wander(b *GoWorld.Being) error {
	dX := math.Sqrt(b.Speed) * (rand.NormFloat64() * 5)
	dY := math.Sqrt(b.Speed) * (rand.NormFloat64() * 5)
	wanderSpot := GoWorld.Location{}
	wanderSpot.X = b.Position.X + int(dX)
	wanderSpot.Y = b.Position.Y + int(dY)

	if wanderSpot.X < 0 {
		wanderSpot.X = 0
	} else if wanderSpot.X >= w.Width {
		wanderSpot.X = w.Width - 1
	}
	if wanderSpot.Y < 0 {
		wanderSpot.Y = 0
	} else if wanderSpot.Y >= w.Height {
		wanderSpot.Y = w.Height - 1
	}

	for !w.canPlaceBeing(wanderSpot, b.Type) {
		dX = math.Sqrt(b.Speed) * (rand.NormFloat64() * 5)
		dY = math.Sqrt(b.Speed) * (rand.NormFloat64() * 5)
		wanderSpot.X = b.Position.X + int(dX)
		wanderSpot.Y = b.Position.Y + int(dY)

		if wanderSpot.X < 0 {
			wanderSpot.X = 0
		} else if wanderSpot.X >= w.Width {
			wanderSpot.X = w.Width - 1
		}
		if wanderSpot.Y < 0 {
			wanderSpot.Y = 0
		} else if wanderSpot.Y >= w.Height {
			wanderSpot.Y = w.Height - 1
		}
	}
	// Update the spot map
	w.TerrainSpots[b.Position.X][b.Position.Y].Being = uuid.Nil
	w.TerrainSpots[wanderSpot.X][wanderSpot.Y].Being = b.ID

	// Tell the being where it is going
	b.Position.X = wanderSpot.X
	b.Position.Y = wanderSpot.Y

	return nil
}

// canPlaceBeing checks if a being can move to that spot
// Returns false if the spot is already occupied by another being or if the surface type does not allow to walk on it
// (e.g. water or mountain peaks)
func (w *RandomWorld) canPlaceBeing(spot GoWorld.Location, beingType string) bool {
	// Is there perhaps another being present?
	if w.TerrainSpots[spot.X][spot.Y].Being == uuid.Nil {
		// Can being move anywhere (Flying)
		if beingType == "Flying" {
			return true
		} else if beingType == "Water" {
			// Water beings can move on water
			spotName, _ := w.GetSurfaceNameAt(spot)
			if spotName == "Water" || spotName == "Grassland" {
				return true
			}
		} else {
			if w.TerrainSpots[spot.X][spot.Y].Surface.Habitable {
				// Spot can be moved on, {
				// No being present and habitable, we can safely move a being to this spot
				return true
			}
		}
	}
	// Spot was not habitable or a being was present
	return false
}

// UpdatePlant updates the attributes for plant. It can grow, produce seeds or wither
// Returns action done as string and list of UUIDs of objects affected by action
func (w *RandomWorld) UpdatePlant(p *GoWorld.Food) (string, []uuid.UUID) {
	// Simulation runs at around 60FPS, so wither 15x per second
	p.Wither -= 1. / 4
	if p.Wither <= 0 {
		// Kill the plant :(
		delete(w.FoodList, p.ID.String())
		w.updatePlantSpot(p.Position.X, p.Position.Y, p.Area, uuid.Nil)
		return "withered", []uuid.UUID{p.ID}
	}
	// Make the plant grow if not in last stage
	if p.GrowthStage <= stageRange.Max {
		p.StageProgress += p.GrowthSpeed
	}
	// If stage progress reaches maximum value, move plant to next stage and produce offspring
	if p.StageProgress >= stageProgressRange.Max {
		// Seeds to disperse are based on current stage (max seeds are dispersed when last stage finished
		seedsProduced := int(p.Seeds * p.GrowthStage / growthRange.Max)
		// Reset stage progress and increase stage -> can get to maxStage+1
		p.StageProgress = 0.0
		p.GrowthStage++
		// Plant some seeds :)
		ids := w.DisperseSeeds(p, seedsProduced)
		// Return
		if len(ids) == 0 {
			return "planted fail", ids
		}
		return "planted seeds", ids
	}

	// If water plant: move the plants slightly in one direction
	if p.Type == "Water" {
		// Move if possible to adjacent field
		direction := directions8[rand.Intn(len(directions8))]
		adjacentSpot := GoWorld.Location{
			X: p.Position.X + direction.X,
			Y: p.Position.Y + direction.Y,
		}
		// Find adjacent spot inside map bounds
		for w.IsOutOfBounds(adjacentSpot) {
			direction = directions8[rand.Intn(len(directions8))]
			adjacentSpot.X = p.Position.X + direction.X
			adjacentSpot.Y = p.Position.Y + direction.Y
		}
		// Try to place plant to adjacent spot, otherwise do nothing
		if w.canPlaceWaterPlant(adjacentSpot.X, adjacentSpot.Y, p.Area, p.ID) {
			// Delete plant from old spot
			w.updatePlantSpot(p.Position.X, p.Position.Y, p.Area, uuid.Nil)
			w.TerrainSpots[p.Position.X][p.Position.Y].Object = uuid.Nil
			// Add plant to adjacent spot
			w.updatePlantSpot(adjacentSpot.X, adjacentSpot.Y, p.Area, p.ID)
			w.TerrainSpots[adjacentSpot.X][adjacentSpot.Y].Object = p.ID
			p.Position.X = adjacentSpot.X
			p.Position.Y = adjacentSpot.Y
		}
	}

	// Default return
	return "grew", []uuid.UUID{}
}

// DisperseSeeds plants seeds within some range from plant
// Returns UUIDs of newly planted plants
func (w *RandomWorld) DisperseSeeds(p *GoWorld.Food, seeds int) []uuid.UUID {
	var producedIDs []uuid.UUID
	spots := w.MidpointCircleAt(p.Position, p.Area+p.SeedDisperse)
	for i := 0; i < seeds; i++ {
		// Create mutated plant, but only the required attributes to check if we can place this plant
		seedling := &GoWorld.Food{ID: uuid.New()}
		seedling.Area = MutateValue(p.Area, p.MutationRate, *areaRange)
		// Find a location around the parent
		// SeedDisperse tells how far away from Parent area a seedling can be placed
		// Create an array of available spots which will be marked as visited (deleted from array)
		unvisitedSpots := make([]int, len(spots))
		for i := range spots {
			unvisitedSpots[i] = i
		}
		// Position in unvisited spots list
		rnd := rand.Intn(len(unvisitedSpots))
		// Unvisited spot index
		spotIdx := unvisitedSpots[rnd]
		foundSpot := true
		for len(unvisitedSpots) > 0 {
			// Spot was not available for plant, remove it from the unvisited array
			// unvisitedSpots = append(unvisitedSpots[:rnd], unvisitedSpots[rnd+1:]...)
			// Cheap way: swap element to delete to last place and keep n-1 elements
			unvisitedSpots[rnd] = unvisitedSpots[len(unvisitedSpots)-1]
			unvisitedSpots = unvisitedSpots[:len(unvisitedSpots)-1]
			if len(unvisitedSpots) == 0 {
				// No suitable spot found
				foundSpot = false
				break
			}

			// Check spot based on water or land plants
			if p.Type == "Water" {
				if ok := w.canPlaceWaterPlant(spots[spotIdx].X, spots[spotIdx].Y, seedling.Area, seedling.ID); ok &&
					w.TerrainSpots[spots[spotIdx].X][spots[spotIdx].Y].Surface.ID == p.Habitat {
					// We can place the plant here -> same habitat as parent and no occupying plants
					foundSpot = true
					break
				}
			} else {
				if ok := w.canPlacePlant(spots[spotIdx].X, spots[spotIdx].Y, seedling.Area); ok &&
					w.TerrainSpots[spots[spotIdx].X][spots[spotIdx].Y].Surface.ID == p.Habitat {
					// We can place the plant here -> same habitat as parent and no occupying plants
					foundSpot = true
					break
				}
			}

			// Pick new spot from unvisited
			rnd = rand.Intn(len(unvisitedSpots))
			spotIdx = unvisitedSpots[rnd]
		}

		if foundSpot {
			// Spot found for our plant! Place it there
			// We can fill in the other parameters for plant
			seedling.GrowthStage = 0.0
			seedling.StageProgress = 0.0
			seedling.SeedDisperse = MutateValue(p.SeedDisperse, p.MutationRate, *disperseRange)
			seedling.Taste = MutateValue(p.Taste, p.MutationRate, *tasteRange)
			seedling.NutritionalValue = MutateValue(p.NutritionalValue, p.MutationRate, *nutritionRange)
			seedling.Seeds = MutateValue(p.Seeds, p.MutationRate, *seedRange)
			seedling.Wither = witherRange.randomFloat()
			seedling.MutationRate = MutateValue(p.MutationRate, p.MutationRate, *mutationRange)
			seedling.GrowthSpeed = MutateValue(p.GrowthSpeed, p.MutationRate, *mutationRange)
			seedling.Type = p.Type

			// Place the plant on the free spot
			w.updatePlantSpot(spots[spotIdx].X, spots[spotIdx].Y, seedling.Area, seedling.ID)
			seedling.Habitat = w.TerrainSpots[spots[spotIdx].X][spots[spotIdx].Y].Surface.ID
			seedling.Position.X = spots[spotIdx].X
			seedling.Position.Y = spots[spotIdx].Y
			// Append to food list
			w.FoodList[seedling.ID.String()] = seedling
			// ... and to return list
			producedIDs = append(producedIDs, seedling.ID)
		}
	}
	return producedIDs
}

// UpdatePlantSpot updates the spot and area with the given plant ID.
// If uuid.Nil is given, it removes the plant from the world
// Does not check if spot is valid, use canPlacePlant for that
func (w *RandomWorld) updatePlantSpot(x, y int, plantDiameter float64, id uuid.UUID) {
	// Set the center of the plant
	w.TerrainSpots[x][y].Object = id

	// Get the circular spots
	circleSpots := w.MidpointCircleAt(GoWorld.Location{X: x, Y: y}, plantDiameter/2)

	// Update the occupying ID of those spots
	for _, spot := range circleSpots {
		w.TerrainSpots[spot.X][spot.Y].OccupyingPlant = id
	}

}

// MutateValue produces a new value from the parent value
// It uses a normal distribution with standard deviation of mutation rate and it does not overflow attribute range
func MutateValue(parentAttribute, mutationRate float64, valueRange attributeRange) float64 {
	modifier := rand.NormFloat64() * mutationRate
	parentAttribute += modifier
	// Check if produced value still in specified range
	if parentAttribute < valueRange.Min {
		parentAttribute = valueRange.Min
	} else if parentAttribute > valueRange.Max {
		parentAttribute = valueRange.Max
	}
	return parentAttribute
}

// Mutate values produces a value between first two parameters with a standard deviation of mutation rate
func MutateValues(value1, value2, mutationRate float64, valueRange attributeRange) float64 {
	// Find out which values are lower bound and which is higher
	low, high := value1, value2
	if value1 > value2 {
		low, high = value2, value1
	}
	// Calculate mutation multiplier
	multiplier := rand.NormFloat64() * mutationRate

	// Calculate the random value between the given values and mutate it
	newValue := (rand.Float64()*high + low) * multiplier
	// Limit the value to the minimum and maximum range
	if newValue < valueRange.Min {
		newValue = valueRange.Min
	} else if newValue > valueRange.Max {
		newValue = valueRange.Max
	}
	return newValue
}

// MidpointCircleAt creates a circle with the provided coordinates as the middle point and the radius.
// Returns a list of locations for the filled circle (including midpoint). If circle extends over world edges, then
// those locations are filtered out
func (w *RandomWorld) MidpointCircleAt(center GoWorld.Location, radius float64) []GoWorld.Location {
	// The final spots
	var circleSpots []GoWorld.Location

	// Round the radius to closest int and initialize x with it
	x := int(math.Round(radius))
	y := 0

	// The midpoint circle algorithm calculates the arc values for octaves and translates onto opposite ones,
	// by using the 2 opposite points as line ends we can fill a circle
	for xi := center.X - x; xi <= center.X+x; xi++ {
		spot := GoWorld.Location{X: xi, Y: center.Y + y}
		if oob := w.IsOutOfBounds(spot); !oob {
			circleSpots = append(circleSpots, spot)
		}
	}
	// Initialize the value of P
	P := 1 - int(math.Round(radius))

	// Loop while we are on the rise
	for x >= y {
		y++
		// Is the mid point inside or on the perimeter?
		if P < 0 {
			// Inside
			P = P + 2*y + 1
		} else {
			// Outside the perimeter
			x--
			P = P + 2*y - 2*x + 1
		}
		if x < y {
			break
		}
		// Store the points
		for xi := -x + center.X; xi <= x+center.X; xi++ {
			spot := GoWorld.Location{X: xi, Y: center.Y + y}
			oppositeSpot := GoWorld.Location{X: xi, Y: center.Y - y}
			// Check if valid spots (not out of bounds) and add them to the list
			if oob := w.IsOutOfBounds(spot); !oob {
				circleSpots = append(circleSpots, spot)
			}
			if oob := w.IsOutOfBounds(oppositeSpot); !oob {
				circleSpots = append(circleSpots, oppositeSpot)
			}
		}
		if x != y {
			// When x == y we reached 45 degrees (octave), the points change
			for xi := -y + center.X; xi <= y+center.X; xi++ {
				spot := GoWorld.Location{X: xi, Y: center.Y + x}
				oppositeSpot := GoWorld.Location{X: xi, Y: center.Y - x}
				// Check if valid spots (not out of bounds) and add them to the list
				if oob := w.IsOutOfBounds(spot); !oob {
					circleSpots = append(circleSpots, spot)
				}
				if oob := w.IsOutOfBounds(oppositeSpot); !oob {
					circleSpots = append(circleSpots, oppositeSpot)
				}
			}
		}
	}
	return circleSpots
}

// canPlacePlant checks if a plant with certain size can grow on the given location
// Couple of rules:
//  - the plant center must be in a habitable zone
//  - the growing area is perceived as a circle around the center, with plant.GrowthArea being the circle diameter
//    for simplicity sake the radius is rounded (meaning we get diameter +- 1 of space used)
//  - the growing circular area is allowed to extend over the viewport or into inhabitable zones
// Method returns false if any of the previous conditions are not fulfilled
func (w *RandomWorld) canPlacePlant(x, y int, plantArea float64) bool {
	// Check if surface allows plants to grow
	if w.TerrainSpots[x][y].Surface.Habitable {
		// Spot can be planted on, is it occupied by a plant?
		if w.TerrainSpots[x][y].OccupyingPlant == uuid.Nil {
			// Current spot is free, check the circle with radius plantArea if enough space provided
			// The radius should always be >= 1
			spots := w.MidpointCircleAt(GoWorld.Location{X: x, Y: y}, plantArea/2)
			for _, spot := range spots {
				if w.TerrainSpots[spot.X][spot.Y].OccupyingPlant != uuid.Nil {
					// Found a plant occupying a spot
					return false
				}
			}
			// The necessary spots are not occupied
			return true
		}
	}
	// Surface not habitable or plant already occupying the necessary area to grow
	return false
}

// Checks if a water plant can be placed onto the coordinates
// Same rules apply as for place plant, only that the surface must be water
func (w *RandomWorld) canPlaceWaterPlant(x, y int, plantArea float64, plantID uuid.UUID) bool {
	if w.TerrainSpots[x][y].Surface.CommonName != "Water" {
		// Non water surface provided
		return false
	}

	if w.TerrainSpots[x][y].OccupyingPlant == uuid.Nil || w.TerrainSpots[x][y].OccupyingPlant == plantID {
		// Get a circular area around the spot (2D, depth not accounted for) and check if a plant is too close
		spots := w.MidpointCircleAt(GoWorld.Location{X: x, Y: y}, plantArea/2)
		for _, spot := range spots {
			// Skip non water surfaces, as we only need points in water
			if w.TerrainSpots[spot.X][spot.Y].Surface.CommonName != "Water" {
				continue
			}
			if w.TerrainSpots[spot.X][spot.Y].OccupyingPlant != uuid.Nil &&
				w.TerrainSpots[x][y].OccupyingPlant != plantID {
				// Found a plant occupying a spot, not enough room for this plant
				return false
			}
		}
		// The necessary spots are not occupied, meaning a plant can live here
		return true
	}
	return false
}

// New returns new terrain generated using Perlin noise
func (w *RandomWorld) New() error {
	// Check if the world was initialized with valid terrain sizes
	if w.Height <= 0 || w.Width <= 0 {
		return fmt.Errorf("the terrain size can't be less than or equal to zero (given WxH: %dx%d)", w.Width,
			w.Height)
	}
	// Initialize the food and being map
	w.BeingList = make(map[string]*GoWorld.Being)
	w.FoodList = make(map[string]*GoWorld.Food)

	// Set the pathfinder
	w.pathFinder = pathing.NewPathfinder(w)

	// Initialize the empty images of the terrain
	rect := image.Rect(0, 0, w.Width, w.Height)
	w.TerrainImage = image.NewGray(rect)
	w.TerrainZones = image.NewRGBA(rect)
	w.TerrainSpots = make([][]*Spot, w.Width)
	for i := range w.TerrainSpots {
		w.TerrainSpots[i] = make([]*Spot, w.Height)
		for j := range w.TerrainSpots[i] {
			w.TerrainSpots[i][j] = &Spot{}
		}
	}

	// Get an instance of a Perlin noise generator
	perl := noise.NewPerlin(6, 0.4, 0)
	var g color.Gray
	var grayNoise uint8
	// Histogram to calculate how many pixels belong to each value (grayscale, so 256 bins with size 1)
	hist := make([]int, 256)
	// Fill the grayscale image with Perlin noise
	for x := 0; x < w.Width; x++ {
		for y := 0; y < w.Height; y++ {
			floatNoise := perl.OctaveNoise2D(float64(x)/255, float64(y)/255)

			// Paint the grayscale (pseudo DEM) terrain
			grayNoise = uint8(floatNoise * 255)
			g = color.Gray{
				Y: grayNoise,
			}
			w.TerrainImage.Set(x, y, g)
			// Increment the bin that counts the grayscale value
			hist[grayNoise]++
		}
	}
	// Calculate at which height (0-255 grayscale) a zone begins and ends with custom ratios for each zone
	zoneLimits := w.CalculateZoneLimits(hist, 0.20, 0.50, 0.10, 0.15, 0.025, 0.025)

	var c color.RGBA
	for x := 0; x < w.Width; x++ {
		for y := 0; y < w.Height; y++ {
			grayNoise = w.TerrainImage.GrayAt(x, y).Y
			// Paint the zones using colors
			for i, l := range zoneLimits {
				if grayNoise <= l {
					// Found the appropriate zone, paint it with the i-th color
					c = Surfaces[i].Color
					w.TerrainSpots[x][y].Surface = &Surfaces[i]
					break
				}
			}
			w.TerrainZones.Set(x, y, c)
		}
	}
	// Store the terrain image
	f, _ := os.Create("terrain.png")
	defer f.Close()
	_ = png.Encode(f, w.TerrainZones)
	return nil
}

// Provide food generates random plants across the terrain
func (w *RandomWorld) ProvideFood(landPlants, waterPlants int) {
	// Initialize each food with random values
	for i := 0; i < landPlants; i++ {
		p := w.RandomPlant(false)
		w.FoodList[p.ID.String()] = p
	}
	for i := 0; i < waterPlants; i++ {
		p := w.RandomPlant(true)
		w.FoodList[p.ID.String()] = p
	}
}

// randomPlant returns a food object with random parameters
func (w *RandomWorld) RandomPlant(inWater bool) *GoWorld.Food {
	f := &GoWorld.Food{ID: uuid.New()}

	// Randomly select attributes
	f.GrowthSpeed = growthRange.randomFloat()
	f.NutritionalValue = nutritionRange.randomFloat()
	f.Taste = tasteRange.randomFloat()
	f.GrowthStage = float64(stageRange.randomInt()) // keep as float for possible future expandability
	f.StageProgress = stageProgressRange.randomFloat()
	f.Area = areaRange.randomFloat()
	f.Seeds = seedRange.randomFloat()
	f.SeedDisperse = disperseRange.randomFloat()
	f.Wither = witherRange.randomFloat()
	f.MutationRate = mutationRange.randomFloat()

	// place the plant onto the map (check if we want a water plant or not
	if inWater {
		f.Type = "Water"
		w.LaunchPlant(f)
	} else {
		f.Type = "Land"
		w.ThrowPlant(f)
	}
	// Tell the plant what habitat it belongs to
	f.Habitat = w.TerrainSpots[f.Position.X][f.Position.Y].Surface.ID

	return f
}

// GetTerrainImage is a getter for the colored terrain (zones)
func (w *RandomWorld) GetTerrainImage() *image.RGBA {
	return w.TerrainZones
}

// GetBeings is a getter for all living beings
func (w *RandomWorld) GetBeings() map[string]*GoWorld.Being {
	return w.BeingList
}

// GetFood is a getter for all the edible food
func (w *RandomWorld) GetFood() map[string]*GoWorld.Food {
	return w.FoodList
}

// GetSize returns the world bounds (width and height)
func (w *RandomWorld) GetSize() (int, int) {
	return w.Width, w.Height
}

// GetSurfaceColorAtSpot returns the color of the surface (aka the zone) at the desired location
func (w *RandomWorld) GetSurfaceColorAtSpot(spot GoWorld.Location) color.RGBA {
	return w.TerrainSpots[spot.X][spot.Y].Surface.Color
}

// GetSurfaceNameAt returns the common name of the surface at the provided location
// Panics if location is out of bound
func (w *RandomWorld) GetSurfaceNameAt(location GoWorld.Location) (string, error) {
	if w.IsOutOfBounds(location) {
		return "", fmt.Errorf(
			"error providing color at spot: the location (%d, %d) is out of bounds. WorldSize (%v, %v)",
			location.X, location.Y, w.Width, w.Height)
	}
	return w.TerrainSpots[location.X][location.Y].Surface.CommonName, nil
}

// GetBeingAt returns the ID of the being at the provided location
// Returns uuid.Nil if no being present
func (w *RandomWorld) GetBeingAt(location GoWorld.Location) (uuid.UUID, error) {
	if w.IsOutOfBounds(location) {
		return uuid.Nil, fmt.Errorf(
			"error providing being at spot: the location (%d, %d) is out of bounds. WorldSize (%v, %v)",
			location.X, location.Y, w.Width, w.Height)
	}
	return w.TerrainSpots[location.X][location.Y].Being, nil
}

// GetFoodWithID returns an item from the food list with given ID.
// Returns nil if ID does not exist
func (w *RandomWorld) GetFoodWithID(id uuid.UUID) *GoWorld.Food {
	if f, ok := w.FoodList[id.String()]; ok {
		return f
	}
	return nil
}

func (w *RandomWorld) GetBeingWithID(id uuid.UUID) *GoWorld.Being {
	if b, ok := w.BeingList[id.String()]; ok {
		return b
	}
	return nil
}

// IsHabitable returns if the provided spot allows movement and seeding plants
func (w *RandomWorld) IsHabitable(location GoWorld.Location) (bool, error) {
	if w.IsOutOfBounds(location) {
		return false, fmt.Errorf(
			"error checking inhabitable spot: the location (%d, %d) is out of bounds. WorldSize (%v, %v)",
			location.X, location.Y, w.Width, w.Height)
	}
	return w.TerrainSpots[location.X][location.Y].Surface.Habitable, nil
}

// SenseActionFor uses the sense range of the being to decide on its next action
// Rules:
//  1. priorities are in this order: drinks, food, mating, stress
//  2. if any value is above threshold prefer its action, in case many are above threshold follow the previous order
//  3. if stress is above threshold and can not eat/drink or mate try to move to natural habitat
//  4. if nothing in sensing range, or all need fulfilled (values at 0) move randomly
// Returns action to do as string and the location it picked for the action
func (w *RandomWorld) SenseActionFor(b *GoWorld.Being) (string, GoWorld.Location) {
	// Get the spots that are visible to the being
	// Vision range is influenced by stress:
	//  a stress value of 0 represents the beings natural senses, stress of maxStress represents sense range * 2
	stressShare := 1 + b.Stress/stressRange.Max
	surroundings := w.MidpointCircleAt(b.Position, b.VisionRange*stressShare)
	// Get the attribute that is most needed (highest threshold value)
	actionToDo := "wander"
	actionThreshold := 0.0
	// Find out which of 3 basic needs has highest threshold (if > 0)
	// If they have the same threshold if will prefer thirst over hunger over child wishes
	if b.Thirst >= b.Hunger {
		// Thirst is more than hunger (if same prefer thirst)
		if b.Thirst >= b.WantsChild {
			// Being needs water more than other basic necessities
			actionToDo = "drink"
			actionThreshold = b.Thirst
		} else {
			// Being wants child more than water
			actionToDo = "mate"
			actionThreshold = b.WantsChild
		}
	} else {
		// Being is needs food more than water
		if b.Hunger >= b.WantsChild {
			// Being has highest need for food
			actionToDo = "eat"
			actionThreshold = b.Hunger
		} else {
			// Being wants to have a child more than food or water
			actionToDo = "mate"
			actionThreshold = b.WantsChild
		}
	}
	// If the highest threshold was 0 reset the action to wander (being has needs fulfilled)
	if actionThreshold <= 0 {
		actionToDo = "wander"
	}

	// Check the surrounding spots for a suitable place to execute the action
	chosenSpot := GoWorld.Location{}
	chosenMetric := 0.0
	spotUnset := true
	for _, spot := range surroundings {
		spotSurface, _ := w.GetSurfaceNameAt(spot)

		switch actionToDo {
		case "drink":
			// Find the closest water spot
			if spotSurface == "Water" {
				if spotUnset {
					// Set the first spot found
					chosenSpot.X = spot.X
					chosenSpot.Y = spot.Y
					chosenMetric = w.Distance(b.Position, spot)
					spotUnset = false
				} else {
					// Check if this spot is closer than the chosen one
					if dist := w.Distance(b.Position, spot); dist < chosenMetric {
						chosenSpot.X = spot.X
						chosenSpot.Y = spot.Y
						chosenMetric = dist
					}
				}
			}
		case "eat":
			// If being is too hungry find closest food, otherwise tastiest
			if w.TerrainSpots[spot.X][spot.Y].Being == uuid.Nil && b.Type != "Carnivore" {
				if foodId := w.TerrainSpots[spot.X][spot.Y].Object; foodId != uuid.Nil {
					if w.FoodList[foodId.String()] == nil {
						// FixME why is nil food on the map?
						//panic(fmt.Errorf("food present on map is not in food list"))
						w.TerrainSpots[spot.X][spot.Y].Object = uuid.Nil
						continue
					}
					// Water beings can only eat seaweed
					if w.FoodList[foodId.String()].Type == "Water" && b.Type != "Water" {
						// Non water beings cannot eat seaweed
						continue
					} else if b.Type == "Water" && w.FoodList[foodId.String()].Type != "Water" {
						// Water beings only eat seaweed
						continue
					}

					// Found food with no being on it
					if spotUnset {
						chosenSpot.X = spot.X
						chosenSpot.Y = spot.Y
						// Being wants something tasty
						// Make a metric combined of taste and age -> older food is even tastier
						// Invert value because we are using a minimization metric for code simplicity
						// The final growth exceeds growthRange.Max for 1 to disperse seeds for last time
						chosenMetric = tasteRange.Max - w.FoodList[foodId.String()].Taste*
							w.FoodList[foodId.String()].GrowthStage/(growthRange.Max+1)
						if b.Hunger >= hungerThreshold {
							// Being is too hungry to care about taste
							chosenMetric = w.Distance(b.Position, spot)
						}
						spotUnset = false
					} else {
						// Convert to minimization problem for code simplicity
						thisMetric := tasteRange.Max - w.FoodList[foodId.String()].Taste*
							w.FoodList[foodId.String()].GrowthStage/(growthRange.Max+1)
						if b.Hunger >= hungerThreshold {
							// Being is too hungry to care about taste
							thisMetric = w.Distance(b.Position, spot)
						}
						// Check if this food is better (closer or tastier depending on being)
						if thisMetric < chosenMetric {
							chosenSpot.X = spot.X
							chosenSpot.Y = spot.Y
							chosenMetric = thisMetric
						}
					}
				}
			} else if b.Type == "Carnivore" && w.TerrainSpots[spot.X][spot.Y].Being != uuid.Nil {
				// Found spot with being: metric is being size -> nutritional value x2
				if spotSurface == "Forest" && w.BeingList[w.TerrainSpots[spot.X][spot.Y].Being.String()].Type == "Flying" {
					// Flying beings hide inside forests and are invisible to predators
					continue
				}
				if b.Type == w.BeingList[w.TerrainSpots[spot.X][spot.Y].Being.String()].Type {
					// We do not encourage cannibalism
					continue
				}

				if spotUnset {
					chosenSpot.X = spot.X
					chosenSpot.Y = spot.Y
					chosenMetric = w.BeingList[w.TerrainSpots[spot.X][spot.Y].Being.String()].Size
					spotUnset = false
					if b.Hunger >= hungerThreshold {
						// Being is too hungry to care about being size
						chosenMetric = w.Distance(b.Position, spot)
					}
				} else {
					newSize := w.BeingList[w.TerrainSpots[spot.X][spot.Y].Being.String()].Size
					if b.Hunger >= hungerThreshold {
						// Being is too hungry to care about being size
						newSize = w.Distance(b.Position, spot)
					}
					// Pick the largest being around
					if newSize > chosenMetric {
						chosenSpot.X = spot.X
						chosenSpot.Y = spot.Y
						chosenMetric = newSize
					}
				}
			} else if b.Type == "Flying" {
				if spotSurface == "Forest" && w.BeingList[w.TerrainSpots[spot.X][spot.Y].Being.String()].Type == "Flying" {
					// Flying beings hide inside forests and are invisible to predators
					continue
				}
				if b.Type == w.BeingList[w.TerrainSpots[spot.X][spot.Y].Being.String()].Type {
					// We do not encourage cannibalism
					continue
				}
				// Flying beings can only eat beings that are at most half their size
				// It can also eat plants -> metric is compared with plant food
				// (Tastiest + oldest plant == largest being)
				if w.BeingList[w.TerrainSpots[spot.X][spot.Y].Being.String()].Size <= b.Size/2 {
					if spotUnset {
						chosenSpot.X = spot.X
						chosenSpot.Y = spot.Y
						// Convert size range to taste range
						// NewValue = (((OldValue - OldMin) * (NewMax - NewMin)) / (OldMax - OldMin)) + NewMin
						chosenMetric = tasteRange.Max - (((w.BeingList[w.TerrainSpots[spot.X][spot.Y].Being.String()].Size -
							sizeRange.Min) * (tasteRange.Max - tasteRange.Min)) / (sizeRange.Max - sizeRange.Min)) + tasteRange.Min
						if b.Hunger >= hungerThreshold {
							// Being is too hungry to care about being size
							chosenMetric = w.Distance(b.Position, spot)
						}
						spotUnset = false
					} else {
						// Minimization problem, so we can also work with plants and their taste levels and also distance
						newSize := tasteRange.Max - (((w.BeingList[w.TerrainSpots[spot.X][spot.Y].Being.String()].Size -
							sizeRange.Min) * (tasteRange.Max - tasteRange.Min)) / (sizeRange.Max - sizeRange.Min)) + tasteRange.Min
						if b.Hunger >= hungerThreshold {
							// Being is too hungry to care about being size
							newSize = w.Distance(b.Position, spot)
						}
						// Pick being if "tastier" than previous beings / plants
						if newSize < chosenMetric {
							chosenSpot.X = spot.X
							chosenSpot.Y = spot.Y
							chosenMetric = newSize
						}
					}
				}
			}
		case "mate":
			// Find the closest being of opposite gender
			if beingID := w.TerrainSpots[spot.X][spot.Y].Being; beingID != uuid.Nil {
				otherBeing := w.BeingList[beingID.String()]
				// Check if other being has a different gender but same type
				if otherBeing.Gender != b.Gender && otherBeing.Type == b.Type {
					if spotUnset {
						// Set the first being
						chosenSpot.X = spot.X
						chosenSpot.Y = spot.Y
						chosenMetric = w.Distance(b.Position, spot)
						spotUnset = false
					} else {
						if dist := w.Distance(b.Position, spot); dist < chosenMetric {
							// This being is closer
							chosenSpot.X = spot.X
							chosenSpot.Y = spot.Y
							chosenMetric = dist
						}
					}
				}
			}
		}
	}
	if spotUnset {
		// No spot was found, meaning surroundings do not offer the desired place
		// Wander and try from next spot
		actionToDo = "wander"
	}
	// Flying or water beings do not need to move to adjacent space to drink, only for mating
	if b.Type == "Carnivore" && actionToDo == "drink" || actionToDo == "mate" {
		// The chosen spot is a spot with surface type water or a being is occupying it, choose any free adjacent spot
		for _, direction := range directions8 {
			adjacentSpot := GoWorld.Location{X: chosenSpot.X + direction.X, Y: chosenSpot.Y + direction.Y}
			if !w.IsOutOfBounds(adjacentSpot) {
				// Spot is not out of bounds
				if w.TerrainSpots[adjacentSpot.X][adjacentSpot.Y].Surface.Habitable &&
					w.TerrainSpots[adjacentSpot.X][adjacentSpot.Y].Being == uuid.Nil {
					// Spot is habitable and not occupied, move to it
					return actionToDo, adjacentSpot
				}
			}
		}
		// No suitable spot was found, wander and try again
		actionToDo = "wander"

	}

	if actionToDo == "wander" {
		// Flags for various actions (predator found or safe spot ...)
		safeSpot := GoWorld.Location{}
		safeSpotFound := false
		hideFromPredator := false
		predatorSpot := GoWorld.Location{}

		// Choose where to wander based on being type
		for _, spot := range surroundings {
			// Check surroundings if any _possible_ predators are nearby and try to escape
			// Predator check is based on being type and size, their hunger is assumed starving
			possiblePredatorID, _ := w.GetBeingAt(spot)
			if possiblePredatorID != uuid.Nil {
				// Check if predator is a carnivore (can definitley eat it) or a flying being twice the size
				predator := w.BeingList[possiblePredatorID.String()]
				// Predators can only hunt other species, cannibalism is not allowed
				if predator.Type != b.Type {
					if predator.Type == "Carnivore" || predator.Type == "Flying" && predator.Size > 2*b.Size {
						hideFromPredator = true
						predatorSpot.X = spot.X
						predatorSpot.Y = spot.Y
					}
				}
			} else {
				// Check if it is a safe spot a.k.a. natural habitat (e.g. flying beings are invisible to predators)
				if w.TerrainSpots[spot.X][spot.Y].Surface.ID == b.Habitat {
					// Should be forest for flying beings and water for water beings
					safeSpot.X = spot.X
					safeSpot.Y = spot.Y
					safeSpotFound = true
				}

			}
			if hideFromPredator && safeSpotFound {
				// We know where to move -> hide in safe spot from predator
				chosenSpot.X = safeSpot.X
				chosenSpot.Y = safeSpot.Y
				break
			}
		}
		if hideFromPredator && !safeSpotFound {
			// We need to RUN from the predator, move in opposite direction on a valid spot

			// The predator directions (+/- in X/Y coordinates)
			predatorDeltaX := predatorSpot.X - b.Position.X
			predatorDeltaY := predatorSpot.Y - b.Position.Y

			// Convert the deltas to -1, 1 or 0 (both can not be zero as the same time)
			if predatorDeltaX < 0 {
				predatorDeltaX = -1
			} else if predatorDeltaX > 0 {
				predatorDeltaX = 1
			}
			if predatorDeltaY < 0 {
				predatorDeltaY = -1
			} else if predatorDeltaY > 0 {
				predatorDeltaY = 1
			}
			// Find a spot in opposite direction od predator deltas and find a *proper* spot (habitable based on being
			// type)
			if predatorDeltaY != 0 && predatorDeltaX != 0 {
				// When both deltas differ from zero we move diagonally
				// Calculate as if the path forms an orthogonal triangle
				// c = sqrt(a^2 + b^2) -> b = sqrt(c^2 - a^2)
				spotsToMoveX := rand.Intn(int(b.Speed))
				spotsToMoveY := int(math.Sqrt(b.Speed*b.Speed - float64(spotsToMoveX)*float64(spotsToMoveX)))
				// Move into opposite directions of deltas
				chosenSpot.X = b.Position.X + (-predatorDeltaX * spotsToMoveX)
				chosenSpot.Y = b.Position.Y + (-predatorDeltaY * spotsToMoveY)

				// Move the chosenSpot inside world bounds
				if chosenSpot.X < 0 {
					chosenSpot.X = 0
				} else if chosenSpot.X >= w.Width {
					chosenSpot.X = w.Width - 1
				}
				if chosenSpot.Y < 0 {
					chosenSpot.Y = 0
				} else if chosenSpot.Y >= w.Height {
					chosenSpot.Y = w.Height - 1
				}
			}
		}
		if !hideFromPredator {
			// Pick a random move spot as we are not running from any predators
			unvisitedSpots := make([]int, len(surroundings))
			for i := range surroundings {
				unvisitedSpots[i] = i
			}
			foundSpot := false
			spotIdx := 0
			rand.Seed(time.Now().Unix())
			rnd := rand.Intn(len(unvisitedSpots))
			for len(unvisitedSpots) > 0 {
				// Position in unvisited spots list
				rnd = rand.Intn(len(unvisitedSpots))
				// Unvisited spot index
				spotIdx = unvisitedSpots[rnd]
				// Spot was not available for plant, remove it from the unvisited array
				// unvisitedSpots = append(unvisitedSpots[:rnd], unvisitedSpots[rnd+1:]...)
				// Cheap way: swap element to delete to last place and keep n-1 elements
				unvisitedSpots[rnd] = unvisitedSpots[len(unvisitedSpots)-1]
				unvisitedSpots = unvisitedSpots[:len(unvisitedSpots)-1]
				if len(unvisitedSpots) == 0 {
					// No suitable spot found
					foundSpot = false
					break
				}
				if w.canPlaceBeing(surroundings[spotIdx], b.Type) {
					// Found a random spot we can move to
					foundSpot = true
					break
				}
			}
			if foundSpot {
				// Found a random move spot
				chosenSpot.X = surroundings[spotIdx].X
				chosenSpot.Y = surroundings[spotIdx].Y

				// If neccessary, try to wander to nautral habitat to lower stress
				if b.Stress >= stressThreshold && safeSpotFound {
					chosenSpot.X = safeSpot.X
					chosenSpot.Y = safeSpot.Y
				}
			}

			//// Whether we found a place to move
			//spotFound := false
			//
			//// Pick a random direction and move there
			//direction := directions8[rand.Intn(8)]
			//// Find out how many spots to move in each direction so that the diagonal is speed
			//spotsToMoveX := rand.Intn(int(b.Speed))
			//spotsToMoveY := int(math.Sqrt(b.Speed * b.Speed - float64(spotsToMoveX) * float64(spotsToMoveX)))
			//
			//// Find a spot inside map bounds
			//newSpot := GoWorld.Location{}
			//newSpot.X = b.Position.X + spotsToMoveX * direction.X
			//newSpot.Y = b.Position.X + spotsToMoveY * direction.Y
			//
			//// Condition that breaks out if not spot is found after some time
			//noinfinite := 30
			//degradedSpeed := int(b.Speed)
			//for w.IsOutOfBounds(newSpot) || !w.canPlaceBeing(newSpot, b.Type) {
			//	// Spot is outside bounds or not habitable for being, check a different direction
			//	direction = directions8[rand.Intn(8)]
			//	spotsToMoveX := rand.Intn(degradedSpeed)
			//	spotsToMoveY := int(math.Sqrt(float64(degradedSpeed) * float64(degradedSpeed) -
			//		float64(spotsToMoveX) * float64(spotsToMoveX)))
			//	newSpot.X = b.Position.X + spotsToMoveX * direction.X
			//	newSpot.Y = b.Position.X + spotsToMoveY * direction.Y
			//	noinfinite--
			//	if degradedSpeed > 2 {
			//		degradedSpeed --
			//	}
			//	if noinfinite < 0 {
			//		fmt.Println("OOB: ", w.IsOutOfBounds(newSpot))
			//		fmt.Println("can not place being: ", !w.canPlaceBeing(newSpot, b.Type))
			//		fmt.Println("No inf break")
			//		spotFound = false
			//		break
			//	}
			//}
			//if spotFound {
			//	// We can wander to a random spot inside speed range
			//	chosenSpot.X = newSpot.X
			//	chosenSpot.Y = newSpot.Y
			//} else {
			//	// No spots in max being speed circle are available (30 checked)
			//	actionToDo = "hold"
			//}

		}
	}
	// Increase being speed if it's ready for the hunt
	if actionToDo == "eat" && b.Type == "Carnivore" {
		// Resets after being tries to eat
		b.Speed *= 2
	}

	return actionToDo, chosenSpot
}

// Distance returns the euclidean distance between two locations. To speed up we leave out the square root
func (w *RandomWorld) Distance(from, to GoWorld.Location) float64 {
	return math.Sqrt(math.Pow(float64(from.X-to.X), 2) + math.Pow(float64(from.Y-to.Y), 2))
}

// MoveBeingToLocation moves the being to the provided location
func (w *RandomWorld) MoveBeingToLocation(b *GoWorld.Being, to GoWorld.Location) error {
	// Check if location is valid for being to move to
	//if ok, err := w.IsHabitable(to); !ok {
	//	panic(err.Error())
	//	return err
	//}
	// Update the terrain spots with the new being
	w.TerrainSpots[b.Position.X][b.Position.Y].Being = uuid.Nil
	w.TerrainSpots[to.X][to.Y].Being = b.ID

	// Update being position
	b.Position.X = to.X
	b.Position.Y = to.Y

	return nil
}

// QuenchThirst tries to drink water if being is located 1 field away from water
// Returns true when being was able to drink, otherwise returns false
func (w *RandomWorld) QuenchThirst(b *GoWorld.Being) bool {
	// Set true if water found
	drank := false
	for _, d := range directions8 {
		// Check if out of bounds
		if b.Position.X+d.X < 0 || b.Position.X+d.X >= w.Width ||
			b.Position.Y+d.Y < 0 || b.Position.Y+d.Y >= w.Height {
			// Not on map, simply continue
			continue
		}
		// Check if surface type is water
		if w.TerrainSpots[b.Position.X+d.X][b.Position.Y+d.Y].Surface.CommonName == "Water" {
			drank = true
			break
		}
	}
	// If Being was able to drink, lower its thirst
	if drank {
		b.Thirst = 0
	}

	return drank
}

// QuenchHunger tries to eat food if being is located on top of or next to food
// For food selection see method RandomWorld.MoveBeingTo()
// Returns true if being ate
func (w *RandomWorld) QuenchHunger(b *GoWorld.Being, foodSpot GoWorld.Location) bool {
	// Adjacent fields and the center point (9 locations)
	// Usually the character moves on top of food when eating it so check 0,0 first

	// Has the Being eaten?
	ate := false
	// Check the distance from food to being
	distanceToFood := w.Distance(b.Position, foodSpot)
	if distanceToFood < 2 {
		// Food spot is an adjacent field, we can eat
		// Do we eat beings or plants?
		if beingID := w.TerrainSpots[foodSpot.X][foodSpot.Y].Being; beingID != uuid.Nil && (b.Type == "Carnivore" ||
			b.Type == "Flying") {

			// Being is present on the spot, EAT IT
			beingToEat := w.BeingList[beingID.String()]
			b.Hunger -= beingToEat.Size * 4 // Nutritional value of being is 4x its size
			ate = true
			delete(w.BeingList, beingID.String())
			w.TerrainSpots[foodSpot.X][foodSpot.Y].Being = uuid.Nil
			if b.Hunger < 0 {
				b.Hunger = 0
			}

		} else {
			// Herbivore: eat plants
			foodID := w.TerrainSpots[foodSpot.X][foodSpot.Y].Object
			if foodID == uuid.Nil {
				fmt.Println("The spot does not have an item for being to eat")
				return false
			}
			if w.FoodList[foodID.String()] == nil {
				fmt.Println("Hmm, terrain spot has food that is not in food list?? Being can not eat here")
				return false
			}
			food := w.FoodList[foodID.String()]
			// Eat the whole thing -> lowers hunger by nutritional value
			b.Hunger -= food.NutritionalValue
			ate = true
			delete(w.FoodList, food.ID.String())
			w.TerrainSpots[food.Position.X][food.Position.Y].Object = uuid.Nil
			w.updatePlantSpot(food.Position.X, food.Position.Y, food.Area, uuid.Nil)

			// Hunger should not be negative
			if b.Hunger < 0 {
				// The being can not eat more, so break out of the loop
				b.Hunger = 0
			}
			// Todo also lower thirst with a small chance
		}
	}

	return ate
}

// AdjustStressFor updates the stress value for the being
// The highest stress (255) is achieved when all basic necessities (food, drinks, mating) are at 255 and being is
// outside its natural habitat zone.
// The basic necessities all give the same amount of stress (multiplier of 0.1667), being outside the natural habitat
// doubles the stress (0.3333 per necessity)
// Higher values of being Size lower stress
func (w *RandomWorld) AdjustStressFor(b *GoWorld.Being) {
	// Does the being feel safe? 1 for yes, 2 for no (e.g. outside natural habitat)
	feelsSafe := 1.0
	if w.TerrainSpots[b.Position.X][b.Position.Y].Surface.ID != b.Habitat {
		feelsSafe = 2.0
	}
	// How much every necessity contributes
	// (2 * len(basicNecessities) * contribution = 2 * 3 * contribution = 1)
	c := 1.0 / 6
	// The biggest beings (terms of size) gets only ~10% of stress compared to smallest being
	sizeC := 1 - b.Size/(sizeRange.Max*1.1)

	// Update stress
	// Fixme somehow goes over 255
	b.Stress = feelsSafe * c * (b.Thirst + b.Hunger + b.WantsChild) * sizeC
	if b.Stress > 255 {
		b.Stress = 255
	}
}

// AdjustNeeds increases the being needs for the current epoch
// Higher values lower need for food / drinks:
//  - Durability
// Higher values increase need for food / drinks:
//  - Speed
//  - Stress
//  - Size
func (w *RandomWorld) AdjustNeeds(b *GoWorld.Being) {
	// Most durable beings (compared to least) need only ~30% food
	// 0.3 = 1 - x / (x*1.43) for any x
	durableC := 1 - b.Durability/(durabilityRange.Max*1.43)

	// Increase other values proportional to attribute shares
	speedC := 1 + b.Speed/(speedRange.Max)
	stressC := 1 + b.Stress/(stressRange.Max)
	sizeC := 1 + b.Size/(sizeRange.Max)
	// Calculate the multiplier for increase per epoch values
	multiplier := durableC * speedC * stressC * sizeC

	// Update the basic needs with the given multiplier
	b.Hunger += hungerIncrease * multiplier

	// Water beings thirst does not increase while in water, but increases twice as fast outside of water
	beingSurface, _ := w.GetSurfaceNameAt(b.Position)
	if b.Type == "Water" && beingSurface != "Water" {
		b.Thirst += thirstIncrease * multiplier * 2
	} else if b.Type != "Water" {
		// Normal increase for other beings
		// Water beings' thirst does not increase while they are in water
		b.Thirst += thirstIncrease * multiplier
	}

	b.WantsChild += wantsChildIncrease
}

// MateBeing tries to mate two adjacent beings with opposite genders and produce offspring
// The mutation rate is taken from the initiator
// Returns IDs of children produced
func (w *RandomWorld) MateBeing(b *GoWorld.Being) []uuid.UUID {
	// Find a partner of opposite gender on adjacent fields
	var otherBeing *GoWorld.Being
	for _, direction := range directions8 {
		adjacentSpot := GoWorld.Location{X: b.Position.X + direction.X, Y: b.Position.Y + direction.Y}
		if !w.IsOutOfBounds(adjacentSpot) {
			// Check if being there
			if beingID := w.TerrainSpots[adjacentSpot.X][adjacentSpot.Y].Being; beingID != uuid.Nil {
				// Check if opposite gender
				if w.BeingList[beingID.String()].Gender != b.Gender {
					// Chose this being to mate with
					otherBeing = w.BeingList[beingID.String()]
					break
				}
			}
		}
	}
	if otherBeing == nil {
		// No adjacent being found, cannot mate
		return []uuid.UUID{}
	}
	var babyIDs []uuid.UUID
	// Both beings are present, make some babies
	babiesToMake := int(MutateValues(b.Fertility, otherBeing.Fertility, b.MutationRate, *fertilityRange))
	for i := 0; i < babiesToMake; i++ {
		babyHasSpot := false
		// Find empty spot first, then create being
		for _, direction := range directions8 {
			adjacentSpot := GoWorld.Location{X: b.Position.X + direction.X, Y: b.Position.Y + direction.Y}
			if w.IsOutOfBounds(adjacentSpot) {
				// Ignore spots out of bounds
				continue
			}
			if w.canPlaceBeing(adjacentSpot, b.Type) {
				// Yay being has spot, give birth to it there
				babyHasSpot = true

				// Create baby from parents values and some mutation
				baby := &GoWorld.Being{ID: uuid.New()}
				baby.Hunger = MutateValues(b.Hunger, otherBeing.Hunger, b.MutationRate, *hungerRange)
				baby.Thirst = MutateValues(b.Thirst, otherBeing.Thirst, b.MutationRate, *thirstRange)
				baby.WantsChild = MutateValues(b.WantsChild, otherBeing.WantsChild, b.MutationRate, *wantsChildRange)
				baby.LifeExpectancy = MutateValues(b.LifeExpectancy, otherBeing.LifeExpectancy, b.MutationRate, *lifeExpectancyRange)
				baby.VisionRange = MutateValues(b.VisionRange, otherBeing.VisionRange, b.MutationRate, *visionRange)
				baby.Speed = MutateValues(b.Speed, otherBeing.Speed, b.MutationRate, *speedRange)
				baby.Durability = MutateValues(b.Durability, otherBeing.Durability, b.MutationRate, *durabilityRange)
				baby.Stress = MutateValues(b.Stress, otherBeing.Stress, b.MutationRate, *stressRange)
				baby.Habitat = b.Habitat
				baby.Gender = randomGender()
				baby.Size = MutateValues(b.Size, otherBeing.Size, b.MutationRate, *sizeRange)
				baby.Fertility = MutateValues(b.Fertility, otherBeing.Fertility, b.MutationRate, *fertilityRange)
				baby.MutationRate = MutateValues(b.MutationRate, otherBeing.MutationRate, b.MutationRate, *mutationRange)
				baby.Position.X = adjacentSpot.X
				baby.Position.Y = adjacentSpot.Y
				baby.Type = b.Type

				// Add the baby to the being list and place on map
				w.TerrainSpots[adjacentSpot.X][adjacentSpot.Y].Being = baby.ID
				w.BeingList[baby.ID.String()] = baby
				babyIDs = append(babyIDs, baby.ID)
			}
		}
		if !babyHasSpot {
			// No more room around parent for new babies
			break
		}
	}
	// If at least one baby was born remove the wish for babies from both beings
	if len(babyIDs) > 0 {
		b.WantsChild = 0
		otherBeing.WantsChild = 0
	}
	return babyIDs
}
