package display

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	"github.com/rubinda/GoWorld"
	"image/color"
)

var (
	world GoWorld.World
	// Gender specific colors (for marking dots on the terrain as beings)
	manBlue color.RGBA = color.RGBA{
		R: 103, G: 175, B: 255, A: 255,
	}
	womanViolet color.RGBA = color.RGBA{
		R: 176, G: 101, B: 255, A: 255,
	}
	alienGreen color.RGBA = color.RGBA{
		R: 0, G: 255, B: 0, A: 255,
	}

	beingSprites map[string]*BeingSprite
	foodSprites  map[string]*FoodSprite

	// Growth stage 4 (final one)
	pumpkin *ebiten.Image
	// Growth stage 3
	corn *ebiten.Image
	// Growth stage 2
	eggplant *ebiten.Image
	// Growth stage 1
	carrot *ebiten.Image
	// Growth stage 0
	potato *ebiten.Image

	// Gender images
	manImage   *ebiten.Image
	womanImage *ebiten.Image

	// Number of updates called
	time uint64
)

// BeingSprite is the image representing a being on the display
type BeingSprite struct {
	Being *GoWorld.Being // The Being this sprite belongs to
	x     int            // Sprite X position on display
	y     int            // Sprite Y position on display
	image *ebiten.Image  // The sprite image
}

type FoodSprite struct {
	Food  *GoWorld.Food // The food object the sprite belongs to
	x     int           // Sprite Y position on the display
	y     int           // Sprite X position on the display
	w     int
	h     int
	image *ebiten.Image // The sprite image
}

// Update on a being Sprite moves it in the world und updates its coordinates
func (bs *BeingSprite) Update() {
	// Make the being do an action in the terrain package
	actionDone, ids := world.UpdateBeing(bs.Being)

	// Check if being died => remove it from sprite list
	switch actionDone {
	case "died":
		delete(beingSprites, ids[0].String())
		return
	case "ate":
		// Remove the food item from screen (being ate it)
		delete(foodSprites, ids[0].String())
	case "mated":
		// Add the new beings to sprites
		for _, id := range ids {
			bs.New(id)
		}
	case "drank":
		// TODO I don't think anything happened with the being?
	}
	// Synchronize the positional coordinates with the terrain package
	bs.x = bs.Being.Position.X
	bs.y = bs.Being.Position.Y
}

// New creates a new food sprite based on ID from GoWorld.Food
func (fs *FoodSprite) New(id uuid.UUID) {
	// Get food from terrain package
	f := world.GetFoodWithID(id)
	foodSprites[id.String()] = &FoodSprite{
		Food:  f,
		x:     f.Position.X,
		y:     f.Position.Y,
		w:     16,
		h:     16,
		image: growthStageImage(f.GrowthStage),
	}
}

// New creates a new being sprite based on being with ID
func (bs *BeingSprite) New(id uuid.UUID) {
	img := manImage
	// Get from terrain package
	b := world.GetBeingWithID(id)
	// Check the sex of the baby
	if b.Gender == "female" {
		img = womanImage
	}
	beingSprites[id.String()] = &BeingSprite{
		Being: b,
		x:     b.Position.X,
		y:     b.Position.Y,
		image: img,
	}
}

func (fs *FoodSprite) Update() {
	actionDone, uuids := world.UpdatePlant(fs.Food)
	// Check what happened with the plant and update sprites accordingly
	switch actionDone {
	case "withered":
		// The plant died :(
		delete(foodSprites, uuids[0].String())
	case "planted seeds":
		// The plant had babies :)
		for _, id := range uuids {
			fs.New(id)
		}
	case "planted fail":
		// Planting failed, but still plant is in new stage
		fs.image = growthStageImage(fs.Food.GrowthStage)
	}
}

// BeingSprites is and array of BeingSprite
type BeingSprites struct {
	array []*BeingSprite // The array containing being sprites
	num   int            // the length of the array
}

// FoodSprites is an array of FoodSprite
type FoodSprites struct {
	array []*FoodSprite // Array containing sprites
	num   int           // array length
}

// Update on BeingSprites calls the update function for every individual sprite
func (bss *BeingSprites) Update() {
	for i := 0; i < bss.num; i++ {
		bss.array[i].Update()
	}
}

// GrowthStageImage returns the image associated with a growth stage
func growthStageImage(stage float64) *ebiten.Image {
	switch s := stage; {
	case s >= 1 && s < 2:
		// Growth stage 1
		return carrot
	case s >= 2 && s < 3:
		// Growth stage 2
		return eggplant
	case s >= 3 && s < 4:
		// Growth stage 3
		return pumpkin
	case s >= 4:
		// Final stage
		return corn
	default:
		// The default image is stage 0 -> potato
		return potato
	}
}

// BeingSpriteInit initializes the being sprites out of beings already present in the world
func BeingSpriteInit() error {
	// Get the beings from the world and create the BeingSprite array of same size
	beings := world.GetBeings()
	if len(beings) == 0 {
		return fmt.Errorf("error initializing being sprites: no beings to map sprites to")
	}
	beingSprites = make(map[string]*BeingSprite)

	// The default color for the being is 'Alien green'
	// This should always change to a specific gender, but just in case ...
	img := manImage

	// Initialize the image we will later color as a simple rectangular sprite
	for _, b := range beings {
		// Check what gender every being is and update the color accordingly
		if b.Gender == "male" {
			img = manImage
		} else if b.Gender == "female" {
			img = womanImage
		}
		// Paint the simple sprite with the gender based color
		beingSprites[b.ID.String()] = &BeingSprite{
			Being: b,
			x:     b.Position.X,
			y:     b.Position.Y,
			image: img,
		}
	}
	return nil
}

func FoodSpriteInit() error {
	// Get food from terrain package
	food := world.GetFood()
	if len(food) == 0 {
		return fmt.Errorf("error initializing food sprites: no food present")
	}
	// Store food sprites into map for easy access
	foodSprites = make(map[string]*FoodSprite)

	for _, f := range food {
		// Create new food sprite
		foodSprites[f.ID.String()] = &FoodSprite{
			Food:  f,
			x:     f.Position.X,
			y:     f.Position.Y,
			w:     16,
			h:     16,
			image: growthStageImage(f.GrowthStage),
		}
	}
	return nil
}

// update is the ebiten function that handles screen drawing updates
func update(screen *ebiten.Image) error {
	// Draw the background colored terrain (zones)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, 0)
	terrainImage, _ := ebiten.NewImageFromImage(world.GetTerrainImage(), ebiten.FilterDefault)
	_ = screen.DrawImage(terrainImage, op)

	if ebiten.IsDrawingSkipped() {
		return nil
	}
	// Draw food onto screen

	for _, f := range foodSprites {
		f.Update()
		op.GeoM.Reset()
		op.GeoM.Translate(float64(f.x-f.w/2), float64(f.y-f.h/2))
		_ = screen.DrawImage(f.image, op)
	}

	// Redraw the sprites on screen to match the new positions
	for _, s := range beingSprites {
		s.Update()
		op.GeoM.Reset()
		op.GeoM.Translate(float64(s.x), float64(s.y))
		// As of ebiten 1.5.0 alpha DrawImage() always returns nil, so safe to ignore return value
		_ = screen.DrawImage(s.image, op)

	}
	time++
	if time == 10000 {
		world.PlantsToJSON("plants@10k.json")
		world.BeingsToJSON("beings@10k.json")
	}
	return nil
}

// checkError panics if error is not nil
func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

// init initializes the image sprites
func init() {
	// Load food sprites for each growth stage
	var err error
	pumpkin, _, err = ebitenutil.NewImageFromFile("assets/pumpkin.png", ebiten.FilterDefault)
	checkError(err)
	potato, _, err = ebitenutil.NewImageFromFile("assets/potato.png", ebiten.FilterDefault)
	checkError(err)
	corn, _, err = ebitenutil.NewImageFromFile("assets/corn.png", ebiten.FilterDefault)
	checkError(err)
	eggplant, _, err = ebitenutil.NewImageFromFile("assets/eggplant.png", ebiten.FilterDefault)
	checkError(err)
	carrot, _, err = ebitenutil.NewImageFromFile("assets/carrot.png", ebiten.FilterDefault)
	checkError(err)

	manImage, err = ebiten.NewImage(10, 10, ebiten.FilterDefault)
	checkError(err)
	err = manImage.Fill(manBlue)
	checkError(err)
	womanImage, err = ebiten.NewImage(10, 10, ebiten.FilterDefault)
	checkError(err)
	err = womanImage.Fill(womanViolet)
	checkError(err)

	// Start time
	time = 0
}

// Run draws the initial terrain
// Provide screen width and height and a initialized world
func Run(goworld GoWorld.World) {
	world = goworld // Set the global world variable
	if err := BeingSpriteInit(); err != nil {
		// TODO handle no beings in the world better than panicing
		panic(err)
	}
	if err := FoodSpriteInit(); err != nil {
		panic(err)
	}
	screenWidth, screenHeight := world.GetSize()
	// Start the display output
	//ebiten.SetMaxTPS(30)
	if err := ebiten.Run(update, screenWidth, screenHeight, 1, "GoWorld"); err != nil {
		panic(err)
	}
}
