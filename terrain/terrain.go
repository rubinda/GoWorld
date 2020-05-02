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
		Surface{uuid.New(), "Sea", color.RGBA{R: 116, G: 167, B: 235, A: 255}, false},
		Surface{uuid.New(), "Grassland", color.RGBA{R: 96, G: 236, B: 133, A: 255}, true},
		Surface{uuid.New(), "Forest", color.RGBA{R: 44, G: 139, B: 54, A: 255}, true},
		Surface{uuid.New(), "Gravel", color.RGBA{R: 198, G: 198, B: 198, A: 255}, true},
		Surface{uuid.New(), "Mountain", color.RGBA{R: 204, G: 153, B: 102, A: 255}, true},
		Surface{uuid.New(), "Moutain Peak", color.RGBA{R: 240, G: 240, B: 240, A: 255},
			false},
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
	sizeRange           = &attributeRange{1, 8}
	fertilityRange      = &attributeRange{0, 7}
	mutationRange       = &attributeRange{0, 31}
)

// RandomWorld represents the world implementation using Perlin Noise as terrain
type RandomWorld struct {
	// TODO introduce MaxBeings based on world size / terrain
	Width, Height int
	TerrainImage  *image.Gray // TerrainImage holds the terrain surface image (like a DEM model)
	TerrainZones  *image.RGBA // TerrainZones is a colored version of TerrainImage (based on defined zones and ratios)
	TerrainSpots  [][]*Spot   // TerrainSpots holds data about each spot on the map (what surface, what object or being
	// occupies it)
	BeingList []*GoWorld.Being // The list of world inhabitants
}

// Spot is a place on the map with a defined surface type.
// Optionally an object (e.g. food) and a being can be located in it (a being above the object, for example eating food)
type Spot struct {
	Surface *Surface  // The surface attributes
	Object  uuid.UUID // The UUID for the object (nil for nothing)
	Being   uuid.UUID // The being on the spot (nil for noone)
}

// Surface represents the data about a certain zone
type Surface struct {
	ID         uuid.UUID // The UUID (e.g. '7d444840-9dc0-11d1-b245-5ffdce74fad2'
	CommonName string    // A common name for it (e.g. 'Forest')
	// TODO use textures instead of colors
	Color    color.RGBA // A color value for the appearance
	Walkable bool       // Whether a Being can move across this surface (e.g. Can't walk on moutain peaks or on water)
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

// randomGender picks a gender with a 50/50 chance
func randomGender() string {
	rand.Seed(time.Now().UnixNano())
	coinFlip := rand.Intn(2)
	if coinFlip > 0 {
		return "female"
	}
	return "male"

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
// Note that the beings are added to the world and previoudly created beings are kept
func (w *RandomWorld) CreateBeings(quantity int) []*GoWorld.Being {
	// TODO Check if the user can fit all these beings onto the terrain
	// Initialize the required quantity of beings
	newBeings := make([]*GoWorld.Being, quantity)

	// Initialize each being to a random one
	for i := range newBeings {
		newBeings[i] = w.CreateRandomBeing()
	}
	// Store these beings into the world
	w.BeingList = append(w.BeingList, newBeings...)

	return newBeings
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

	// Specify into which habitat (surface type) it falls
	b.Habitat = w.TerrainSpots[rX][rY].Surface.ID
}

// MoveBeing moves a being according to Brownian Motion
// Implementation reference: http://people.bu.edu/andasari/courses/stochasticmodeling/lecture5/stochasticlecture5.html
// I have adjusted the following parameters:
//  - the time step (delta t) is the speed of each creature
//  - the previous position is the current position of the being
//  - the next position is recalculated until a valid one is found
func (w *RandomWorld) MoveBeing(b *GoWorld.Being) error {
	dX := math.Sqrt(b.Speed) * rand.NormFloat64()
	dY := math.Sqrt(b.Speed) * rand.NormFloat64()
	newX := b.Position.X + int(dX)
	newY := b.Position.Y + int(dY)

	if newX < 0 {
		newX = 0
	} else if newX >= w.Width {
		newX = w.Width-1
	}
	if newY < 0 {
		newY = 0
	} else if newY >= w.Height {
		newY = w.Height-1
	}

	for !w.canPlaceBeing(newX, newY) {
		dX = math.Sqrt(b.Speed) * rand.NormFloat64()
		dY = math.Sqrt(b.Speed) * rand.NormFloat64()
		newX = b.Position.X + int(dX)
		newY = b.Position.Y + int(dY)

		if newX < 0 {
			newX = 0
		} else if newX >= w.Width {
			newX = w.Width-1
		}
		if newY < 0 {
			newY = 0
		} else if newY >= w.Height {
			newY = w.Height-1
		}
	}

	b.Position.X = newX
	b.Position.Y = newY
	return nil
}

// canPlaceBeing checks if a being can move to that spot
// Returns false if the spot is already occupied by another being or if the surface type does not allow to walk on it
// (e.g. water or mountain peaks)
func (w *RandomWorld) canPlaceBeing(x, y int) bool {
	// First check is the surface allows movement
	if w.TerrainSpots[x][y].Surface.Walkable {
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

// New returns new terrain generated using Perlin noise
func (w *RandomWorld) New() error {
	// Check if the world was initialized with valid terrain sizes
	if w.Height <= 0 || w.Width <= 0 {
		return fmt.Errorf("the terrain size can't be less than or equal to zero (given WxH: %dx%d)", w.Width,
			w.Height)
	}
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

	_ = w.CreateBeings(10)

	f, _ := os.Create("terrain.png")
	defer f.Close()
	_ = png.Encode(f, w.TerrainImage)
	return nil
}

// GetTerrainImage is a getter for the colored terrain (zones)
func (w *RandomWorld) GetTerrainImage() *image.RGBA {
	return w.TerrainZones
}

// GetBeings is a getter for all living beings
func (w *RandomWorld) GetBeings() []*GoWorld.Being {
	return w.BeingList
}

// GetSurfaceColorAtSpot returns the color of the surface (aka the zone) at the desired location
func (w *RandomWorld) GetSurfaceColorAtSpot(spot GoWorld.Location) color.RGBA {
	return w.TerrainSpots[spot.X][spot.Y].Surface.Color
}
