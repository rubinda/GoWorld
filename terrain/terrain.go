package terrain

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/rubinda/GoWorld"
	"github.com/rubinda/GoWorld/noise"
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
		{uuid.New(), "Sea", color.RGBA{R: 116, G: 167, B: 235, A: 255}, false},
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
	lifeExpectancyRange = &attributeRange{1, 63}
	visionRange         = &attributeRange{2, 31}
	speedRange          = &attributeRange{1, 31}
	durabilityRange     = &attributeRange{0, 255}
	stressRange         = &attributeRange{0, 255}
	sizeRange           = &attributeRange{1, 2}
	fertilityRange      = &attributeRange{0, 7}
	mutationRange       = &attributeRange{0, 31}

	// Attribute ranges for food
	growthRange    = &attributeRange{0, 15}
	nutritionRange = &attributeRange{0, 255}
	tasteRange     = &attributeRange{0, 255}
	stageRange     = &attributeRange{0, 3}
	areaRange      = &attributeRange{1, 32}
	seedRange      = &attributeRange{1, 16}
	witherRange    = &attributeRange{1, 256}
	disperseRange  = &attributeRange{1, 256}

	// Being thresholds for action
	thirstThreshold     = 100.
	hungerThreshold     = 150.
	wantsChildThreshold = 200.

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
	BeingList map[string]*GoWorld.Being // The list of world inhabitants
	FoodList  map[string]*GoWorld.Food  // List of all edible food
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
	Color       color.RGBA // A color value for the appearance
	Inhabitable bool       // Whether a Being can move across this surface (e.g. Can't walk on moutain peaks or on
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

// IsOutOfBounds check if a location is inside the terrain zone
func (w *RandomWorld) IsOutOfBounds(location *GoWorld.Location) bool {
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

// CreateBeings generates instances of beings and fills them with random attributes
// Provide the number of beings to create
// Note that the beings are added to the world and previously created beings are kept
func (w *RandomWorld) CreateBeings(quantity int) {
	// TODO Check if the user can fit all these beings onto the terrain
	// Initialize each being to a random one
	for i := 0; i < quantity; i++ {
		// Create random being and place it into the map
		b := w.CreateRandomBeing()
		w.BeingList[b.ID.String()] = b
	}
}

// CreateRandomBeing returns a new being with random parameters (places it onto the map)
func (w *RandomWorld) CreateRandomBeing() *GoWorld.Being {
	// Create an empty being
	being := &GoWorld.Being{ID: uuid.New()}

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

// ThrowBeing randomly places the a being onto the map (onto walkable surfaces)
// Use with caution as it adjusts the beings habitat to that spot
func (w *RandomWorld) ThrowBeing(b *GoWorld.Being) {
	// Check if the terrain to place the being exists
	if w.TerrainSpots == nil {
		panic(fmt.Errorf("error while creating being: no terrain to place being on"))
	}

	// Create some random coordinates within the world limits
	rX := rand.Intn(w.Width)
	rY := rand.Intn(w.Height)

	// Check if the chosen spot was valid (no being already present and surface is walkable)
	// If not repeat the random process until we find a suitable spot
	for !w.canPlaceBeing(rX, rY) {
		rX = rand.Intn(w.Width)
		rY = rand.Intn(w.Height)
	}
	// Set the location of the being
	b.Position.X = rX
	b.Position.Y = rY
	w.TerrainSpots[rX][rY].Being = b.ID

	// Specify into which habitat (surface type) it falls
	b.Habitat = w.TerrainSpots[rX][rY].Surface.ID
}

// ThrowPlant randomly places a plant (food) onto the map
func (w *RandomWorld) ThrowPlant(p *GoWorld.Food) {
	// Check if the terrain to place the being exists
	if w.TerrainSpots == nil {
		panic(fmt.Errorf("error while creating being: no terrain to place being on"))
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

// UpdateBeing decides what the next move of a being is going to be (based on sense whether to eat drink, reproduce)
// Rules to follow:
//  1. most important factor is water -> if deciding between multiple quench thirst first
//  2. food is second most important
//  3. after that reproduction
//  4. a being does not know how much life it has left (only the observer knows) -> should not affect decisions
//  5. move out of natural habitat only when stress levels low enough
func (w *RandomWorld) UpdateBeing(b *GoWorld.Being) {
	// Check if it is time for the being to die
	if b.LifeExpectancy <= 0 {
		// Being has reached EOL
		// TODO kill it
		// remove being from BeingList & TerrainSpots
		return
	}
	// Increase the age (== lower life expectancy for 1 epoch)
	b.LifeExpectancy -= 1
	// Set when Being has no more actions remaining (can either drink / eat / move or mate in one epoch)
	actionDone := false

	if b.Thirst >= thirstThreshold {
		// TODO drink water / move toward water (or random if not in sensing range)
		// on success: Lowers thirst
		w.QuenchThirst(b)
		actionDone = true
	} else if b.Hunger >= hungerThreshold {
		// TODO eat food / move toward food (or random if not in sensing range)
		// on success: Lowers hunger and (small chance to lower thirst)
		actionDone = true
	} else if b.WantsChild >= wantsChildThreshold {
		// TODO reproduce / find a mate nearby
		// on success: slightly increase hunger, thirst
		actionDone = true
	}
	if !actionDone {
		// Move randomly across the map if no action has been made
		_ = w.RandomMoveBeing(b)
	}

	// Update stress:
	//  increase for higher thirst, hunger and the wish to reproduce, out of natural habitat
	//  lower for higher size, durability
	// TODO UpdateStress(being)
	// Based on stress, higher stress means more sensing range
	// TODO UpdateSenseRange(being)
}

// RandomMoveBeing moves a being according to Brownian Motion
// Implementation reference: http://people.bu.edu/andasari/courses/stochasticmodeling/lecture5/stochasticlecture5.html
// I have adjusted the following parameters:
//  - the time step (delta t) is the speed of each creature
//  - the previous position is the current position of the being
//  - the next position is recalculated until a valid one is found
func (w *RandomWorld) RandomMoveBeing(b *GoWorld.Being) error {
	dX := math.Sqrt(b.Speed) * rand.NormFloat64()
	dY := math.Sqrt(b.Speed) * rand.NormFloat64()
	newX := b.Position.X + int(dX)
	newY := b.Position.Y + int(dY)

	if newX < 0 {
		newX = 0
	} else if newX >= w.Width {
		newX = w.Width - 1
	}
	if newY < 0 {
		newY = 0
	} else if newY >= w.Height {
		newY = w.Height - 1
	}

	for !w.canPlaceBeing(newX, newY) {
		dX = math.Sqrt(b.Speed) * rand.NormFloat64()
		dY = math.Sqrt(b.Speed) * rand.NormFloat64()
		newX = b.Position.X + int(dX)
		newY = b.Position.Y + int(dY)

		if newX < 0 {
			newX = 0
		} else if newX >= w.Width {
			newX = w.Width - 1
		}
		if newY < 0 {
			newY = 0
		} else if newY >= w.Height {
			newY = w.Height - 1
		}
	}
	// Update the spot map
	w.TerrainSpots[b.Position.X][b.Position.Y].Being = uuid.Nil
	w.TerrainSpots[newX][newY].Being = b.ID

	// Tell the being where it is going
	b.Position.X = newX
	b.Position.Y = newY
	return nil
}

// canPlaceBeing checks if a being can move to that spot
// Returns false if the spot is already occupied by another being or if the surface type does not allow to walk on it
// (e.g. water or mountain peaks)
func (w *RandomWorld) canPlaceBeing(x, y int) bool {
	// First check is the surface allows movement
	if w.TerrainSpots[x][y].Surface.Inhabitable {
		// Spot can be moved on, is there perhaps another being present?
		if w.TerrainSpots[x][y].Being == uuid.Nil {
			// No being present and walkable, we can safely move a being to this spot
			// TODO does the being jump to that spot or can a spot on the path to there block it?
			return true
		}
	}
	// Spot was not walkable or a being was present
	return false
}

// UpdatePlantSpot updates the spot and area with the given plant ID.
// If uuid.Nil is given, it removes the plant from the world
func (w *RandomWorld) updatePlantSpot(x, y int, plantDiameter float64, id uuid.UUID) {
	// Set the center of the plant
	w.TerrainSpots[x][y].Object = id

	// Radius is rounded, so we get area diameter +-1 (close enough)
	r := int(math.Round(plantDiameter / 2))
	// My 'sophisticated' for loops to check a circular area around the center point (x,y)
	// FIXME this function checks a diamond not a circle ...
	// Notes:
	//  - allows the plant to extend over terrain image edge
	//  - allows the plant to have its growing area on inhabitable zones (but not the center)
	var sx int // sx or signedX takes care of sign reversal when cx moves from negative to positive
	for cx := -r; cx <= r; cx++ {
		// circleX ranges [-r, r]
		if cx+x < 0 || cx+x >= w.Width {
			// Allow the plant to grow outside the viewport (widthwise)
			continue
		}
		sx = x
		if cx < 0 {
			// Adjust the sign of x when negative (simplifies the code as only 1 for loop is necessary
			sx = -x
		}
		// Y should run based on X, when X = -r, Y should be just one point (X-R, Y),
		// when X = -R+1, Y has 3 possible values: (X-R+1, Y-1), (X-R+1, Y), (X-R+1, Y+1), and so on for
		// X = ...
		for cy := -(r - sx); cy <= r-sx; cy++ {
			if y+cy < 0 || y+cy >= w.Height {
				// Allow the plant to grow outside the viewport (heightwise)
				continue
			}
			// Set the occupying plant
			w.TerrainSpots[x+cx][y+cy].OccupyingPlant = id
		}
	}
}

// canPlacePlant checks if a plant with certain size can grow on the given location
// Couple of rules:
//  - the plant center must be in a inhabitable zone
//  - the growing area is perceived as a circle around the center, with plant.GrowthArea being the circle diameter
//    for simplicity sake the radius is rounded (meaning we get diameter +- 1 of space used)
//  - the growing circular area is allowed to extend over the viewport or into inhabitable zones
// Method returns false if any of the previous conditions are not fulfilled
func (w *RandomWorld) canPlacePlant(x, y int, plantArea float64) bool {
	// Check if surface allows plants to grow
	if w.TerrainSpots[x][y].Surface.Inhabitable {
		// Spot can be planted on, is it occupied by a plant?
		if w.TerrainSpots[x][y].OccupyingPlant == uuid.Nil {
			// Current spot is free, check the circle with radius plantArea if enough space provided
			// For simplicity rename some variables and round the area
			r := int(math.Round(plantArea / 2)) // the circle radius (round for closest value)
			// My 'sophisticated' for loops to check a circular area around the center point (x,y)
			// FIXME this function checks a diamond not a circle ...
			// Notes:
			//  - allows the plant to extend over terrain image edge
			//  - allows the plant to have its growing area on inhabitable zones (but not the center)
			var sx int // sx or signedX takes care of sign reversal when cx moves from negative to positive
			for cx := -r; cx <= r; cx++ {
				// circleX ranges [-r, r]
				if cx+x < 0 || cx+x >= w.Width {
					// Allow the plant to grow outside the viewport (widthwise)
					continue
				}
				sx = x
				if cx < 0 {
					// Adjust the sign of x when negative (simplifies the code as only 1 for loop is necessary
					sx = -x
				}
				// Y should run based on X, when X = -r, Y should be just one point (X-R, Y),
				// when X = -R+1, Y has 3 possible values: (X-R+1, Y-1), (X-R+1, Y), (X-R+1, Y+1), and so on for
				// X = ...
				for cy := -(r - sx); cy <= r-sx; cy++ {
					if y+cy < 0 || y+cy >= w.Height {
						// Allow the plant to grow outside the viewport (heightwise)
						continue
					}
					if w.TerrainSpots[x+cx][y+cy].OccupyingPlant != uuid.Nil {
						return false
					}
				}
			}
			// The whole area was found with no occupying plants, we can plant here
			return true
		}
	}
	// Surface not inhabitable or plant already occupying the necessary area to grow
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
	zoneLimits := w.CalculateZoneLimits(hist, 0.40, 0.35, 0.15, 0.025, 0.025, 0.05)

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
func (w *RandomWorld) ProvideFood(quantity int) {
	// Initialize each food with random values
	for i := 0; i < quantity; i++ {
		p := w.RandomPlant()
		w.FoodList[p.ID.String()] = p
	}
}

// randomPlant returns a food object with random parameters
func (w *RandomWorld) RandomPlant() *GoWorld.Food {
	f := &GoWorld.Food{ID: uuid.New()}

	// Randomly select attributes
	f.GrowthSpeed = growthRange.randomFloat()
	f.NutritionalValue = nutritionRange.randomFloat()
	f.Taste = tasteRange.randomFloat()
	f.GrowthStage = float64(stageRange.randomInt()) // keep as float for possible future expandability
	f.Area = areaRange.randomFloat()
	f.Seeds = seedRange.randomFloat()
	f.SeedDisperse = disperseRange.randomFloat()
	f.Wither = witherRange.randomFloat()

	// place the plant onto the map
	w.ThrowPlant(f)

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

// MoveBeingTo moves the being closer to the Spot
// Rules:
//  - check Object first, move toward it (e.g. move to edible food)
//  - check Being Second (move to a adjacent field of the being)
//  - check Surface third (e.g. water to take a drink)
// Returns an error if for some reason we can not move to that place
// FIXME currently we assume the being can get to the place in one move, meaning: maxSenseDistance = speed, implement a
// 	pathing algorithm and move just speed fields closer to the desired spot
func (w *RandomWorld) MoveBeingTo(b *GoWorld.Being, spot *Spot) error {
	if spot.Object != uuid.Nil {
		// Check where to object is located and move on top of it
		food := w.FoodList[spot.Object.String()]
		w.TerrainSpots[b.Position.X][b.Position.Y].Being = uuid.Nil
		if w.TerrainSpots[food.Position.X][food.Position.Y].Being != uuid.Nil {
			// Other being is already present
			return fmt.Errorf("error moving being to spot: another being is already present")
		}
		// Occupy spot
		w.TerrainSpots[food.Position.X][food.Position.Y].Being = b.ID
		b.Position.X = food.Position.X
		b.Position.Y = food.Position.Y
		return nil
	} else if spot.Being != uuid.Nil {
		// Pick any (suitable) adjacent field
		otherBeing := w.BeingList[spot.Being.String()]
		var chosenSpot *GoWorld.Location
		for _, d := range directions8 {
			chosenSpot = &GoWorld.Location{X: otherBeing.Position.X + d.X, Y: otherBeing.Position.Y + d.Y}
			if !w.IsOutOfBounds(chosenSpot) {
				// Check if surface is walkable and no being present
				if w.TerrainSpots[chosenSpot.X][chosenSpot.Y].Surface.Inhabitable &&
					w.TerrainSpots[chosenSpot.X][chosenSpot.Y].Being == uuid.Nil {
					// Safe to move there
					w.TerrainSpots[b.Position.X][b.Position.Y].Being = uuid.Nil
					w.TerrainSpots[chosenSpot.X][chosenSpot.Y].Being = b.ID
					b.Position.X = chosenSpot.X
					b.Position.Y = chosenSpot.Y
					return nil
				}
			}
		}
		// If we reach this point no suitable point was found, inform by error
		return fmt.Errorf("error moving being to spot: the other being has no suitable adjacent fields")
	} else if spot.Surface.ID != uuid.Nil {
		// Move to certain surface
		// Should be always water
		// TODO use pathing algorithm to find closest water source and move there
		return fmt.Errorf("error moving being to spot: moving to surface type not implemented")
	}
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
		if w.TerrainSpots[b.Position.X+d.X][b.Position.Y+d.Y].Surface.CommonName == "water" {
			drank = true
			break
		}
	}
	// If Being was able to drink, lower its thirst
	// TODO being should take sips not lower its thirst completely in one move
	if drank {
		b.Thirst = 0
	}

	return drank
}

// QuenchHunger tries to eat food if being is located on top of or next to food
// For food selection see method RandomWorld.MoveBeingTo()
// Returns true if being ate
func (w *RandomWorld) QuenchHunger(b *GoWorld.Being) bool {
	// Adjacent fields and the center point (9 locations)
	// Usually the character moves on top of food when eating it so check 0,0 first

	// Has the Being eaten?
	ate := false
	// Check every adjacent field if the being is able to eat
	for _, d := range directions9 {
		// Check if out of bounds
		// Check if out of bounds
		if b.Position.X+d.X < 0 || b.Position.X+d.X >= w.Width ||
			b.Position.Y+d.Y < 0 || b.Position.Y+d.Y >= w.Height {
			// Not on map, simply continue
			continue
		}

		if id := w.TerrainSpots[b.Position.X+d.X][b.Position.Y+d.Y].Object; id != uuid.Nil {
			// Get the food with this ID
			// TODO Can there be an object that is not food?
			food := w.FoodList[id.String()]

			// Eat the whole thing -> lowers hunger by nutritional value
			b.Hunger -= food.NutritionalValue
			ate = true
			// Remove the plant as the being ate the whole thing ... in one bite
			// TODO if plant lowers hunger beyond zero keep the plant and lower its nutrition value?
			delete(w.FoodList, id.String())
			w.updatePlantSpot(food.Position.X, food.Position.Y, food.Area, uuid.Nil)

			// Hunger should not be negative
			if b.Hunger < 0 {
				// The being can not eat more, so break out of the loop
				b.Hunger = 0
				break
			}
			// Todo also lower thirst with a small chance
		}
	}
	return ate
}
